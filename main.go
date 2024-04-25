package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"os"
	"os/signal"
	"time"

	"github.com/DIMO-Network/edge-network/internal/gateways"
	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/hw/linux/btmgmt"
	"github.com/sirupsen/logrus"

	"github.com/muka/go-bluetooth/hw"
)

var Version = "development"

const bleUnsupportedHW = "5.2"

var unitID uuid.UUID
var name string

var btManager btmgmt.BtMgmt

func main() {
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Str("version", Version).
		Logger().
		Output(zerolog.ConsoleWriter{Out: os.Stdout})

	if len(os.Args) > 1 {
		// this is necessary for the salt stack to correctly update and download the edge-network binaries. See README
		s := os.Args[1]
		if s == "-v" {
			// need to print it very simple for salt stack to get
			fmt.Printf("Version: %s \n", Version)
			os.Exit(0)
		}
	}

	name, unitID = commands.GetDeviceName(logger)
	logger.Info().Msgf("SerialNumber Number: %s", unitID)
	ethAddr, ethErr := commands.GetEthereumAddress(unitID)

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	subcommands.Register(&scanVINCmd{unitID: unitID, logger: logger}, "decode loggers")
	subcommands.Register(&buildInfoCmd{logger: logger}, "info")
	subcommands.Register(&canDumpCmd{unitID: unitID}, "canDump operations")

	if len(os.Args) > 1 {
		ctx := context.Background()
		flag.Parse()
		os.Exit(int(subcommands.Execute(ctx)))
	}

	logger.Info().Msgf("Starting DIMO Edge Network")

	coldBoot, err := isColdBoot(logger, unitID)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to get power management status: %s", err)
	}
	logger.Info().Msgf("Bluetooth name: %s", name)
	logger.Info().Msgf("Version: %s", Version)
	// Used by go-bluetooth, and we use this to set how much it logs. Not for this project.
	logrus.SetLevel(logrus.InfoLevel)
	// temporary for us, for release want info level - todo make configurable via cli?
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	hwRevision, err := commands.GetHardwareRevision(unitID)
	if err != nil {
		logger.Err(err).Msg("error getting hardware rev")
	}
	logger.Info().Msgf("hardware version found: %s", hwRevision)

	if ethAddr == nil {
		if ethErr != nil {
			logger.Err(ethErr).Msg("eth addr error")
		}
		logger.Fatal().Msgf("could not get ethereum address")
	} else {
		logger.Info().Msgf("Device Ethereum Address: %s", ethAddr.Hex())
	}

	lss := loggers.NewTemplateStore()

	// get vehicle definitions from Identity API service
	vehicleDefinition := getVehicleInfo(logger, ethAddr)
	if vehicleDefinition != nil {
		logger.Info().Msgf("identity-api vehicle info: %+v", vehicleDefinition)
		vehInfoErr := lss.WriteVehicleInfo(*vehicleDefinition)
		if vehInfoErr != nil {
			logger.Err(vehInfoErr).Msg("error writing vehicle info to tmp cache")
		}
	} else {
		logger.Info().Msg("unable to get vehicle info from identity-api")
		vehicleDefinition, err = lss.ReadVehicleInfo()
		if err != nil {
			logger.Err(err).Msg("no vehicle info found in tmp file cache")
		}
	}

	// OBD / CAN Loggers
	ds := network.NewDataSender(unitID, *ethAddr, logger, vehicleDefinition)
	if ethErr != nil {
		logger.Info().Msgf("error getting ethereum address: %s", err)
		_ = ds.SendErrorPayload(errors.Wrap(ethErr, "could not get device eth addr"), nil)
	}
	vinLogger := loggers.NewVINLogger(logger)
	vehicleSignalDecodingAPI := gateways.NewVehicleSignalDecodingAPIService()
	vehicleTemplates := internal.NewVehicleTemplates(logger, vehicleSignalDecodingAPI, lss)

	// get the VIN, since dependency for other stuff. we want to use the last known query to reduce unnecessary OBD calls & speed it up
	// todo - is this what we want here? shouldn't it be fingerprint here?
	vin := getVIN(lss, vinLogger, logger, ds)

	// if hw revision is anything other than 5.2, setup BLE
	if hwRevision != bleUnsupportedHW {
		err = setupBluez(name)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to setup BlueZ: %s", err)
		}
		app, cancel, obCancel := setupBluetoothApplication(logger, coldBoot, vinLogger, lss)
		defer app.Close()
		defer cancel()
		defer obCancel()
	}
	// todo v2: what if vehicle hasn't been paired yet? so we don't have the ethAddr to DD mapping in backend, if have VIN this should help
	// what about having default settings? But they may not work if we can't even get VIN.
	pids, deviceSettings, err := vehicleTemplates.GetTemplateSettings(ethAddr, vin)
	if err != nil {
		logger.Err(err).Msg("unable to get device settings (pids, dbc, settings)")
		// todo send mqtt error payload reporting this, should have own topic for errors
	}
	if pids != nil {
		pj, err := json.Marshal(pids)
		if err != nil {
			logger.Info().RawJSON("pids", pj).Msg("pids pulled from config")
		}
	}
	if deviceSettings != nil {
		ds, err := json.Marshal(deviceSettings)
		if err != nil {
			logger.Info().RawJSON("deviceSettings", ds).Msg("device settings pulled from config")
		}
	}

	// todo v2: way to enable/disable our own logger engine - should be base on settings by eth addr that we pull from cloud

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fingerprintRunner := internal.NewFingerprintRunner(unitID, vinLogger, ds, lss, logger)

	// query imei
	imei, err := commands.GetIMEI(unitID)
	if err != nil {
		logger.Err(err).Msg("unable to get imei")
	}
	logger.Info().Msgf("imei: %s", imei)

	deviceConf := internal.Device{
		UnitID:          unitID,
		SoftwareVersion: Version,
		HardwareVersion: hwRevision,
		IMEI:            imei,
	}
	// Execute Worker in background.
	runnerSvc := internal.NewWorkerRunner(ethAddr, lss, ds, logger, fingerprintRunner, pids, deviceSettings, deviceConf)
	runnerSvc.Run() // not sure if this will block always. if it does do we need to have a cancel when catch os.Interrupt, ie. stop tasks?

	sig := <-sigChan
	logger.Info().Msgf("Terminating from signal: %s", sig)
}

func setupBluez(name string) error {
	btManager = *hw.NewBtMgmt(adapterID)

	// Need to turn off the controller to be able to modify the next few settings.
	err := btManager.SetPowered(false)
	if err != nil {
		return fmt.Errorf("failed to power off the controller: %w", err)
	}

	err = btManager.SetLe(true)
	if err != nil {
		return fmt.Errorf("failed to enable LE: %w", err)
	}

	err = btManager.SetBredr(false)
	if err != nil {
		return fmt.Errorf("failed to disable BR/EDR: %w", err)
	}

	err = btManager.SetSc(true)
	if err != nil {
		return fmt.Errorf("failed to enable Secure Connections: %w", err)
	}

	err = btManager.SetName(name)
	if err != nil {
		return fmt.Errorf("failed to set name: %w", err)
	}

	err = btManager.SetPowered(true)

	if err != nil {
		return fmt.Errorf("failed to power on the controller: %w", err)
	}

	return nil
}

// getVIN reads persisted VIN if any. If no VIN previously persisted, queries for it. For this reason, It is important
// that when pairing new vehicle we delete the persisted VIN. Fingerprint will catch any mismatching VIN (eg. user connected to different car)
func getVIN(lss loggers.TemplateStore, vinLogger loggers.VINLogger, logger zerolog.Logger, ds network.DataSender) *string {
	vinConfig, _ := lss.ReadVINConfig()
	if vinConfig != nil {
		return &vinConfig.VIN
	}
	vinResp, vinErr := vinLogger.GetVIN(unitID, nil)
	if vinErr != nil {
		logger.Err(vinErr).Msg("error getting VIN")
		_ = ds.SendErrorPayload(errors.Wrap(vinErr, "could not get VIN"), nil)
	} else {
		writeVinErr := lss.WriteVINConfig(models.VINLoggerSettings{
			VIN:              vinResp.VIN,
			VINQueryName:     vinResp.QueryName,
			VINLoggerVersion: 1,
		})
		if writeVinErr != nil {
			logger.Err(writeVinErr).Msg("error writing VIN config")
		}
		return &vinResp.VIN
	}
	return nil
}

func getVehicleInfo(logger zerolog.Logger, ethAddr *common.Address) *models.VehicleInfo {
	identityAPIService := gateways.NewIdentityAPIService(logger)
	vehicleDefinition, err := gateways.Retry[models.VehicleInfo](3, 1*time.Second, logger, func() (interface{}, error) {
		return identityAPIService.QueryIdentityAPIForVehicle(*ethAddr)
	})

	if err != nil {
		logger.Err(err).Msg("failed to get vehicle definitions")
	}

	return vehicleDefinition
}

// Utility Function
func isColdBoot(logger zerolog.Logger, unitID uuid.UUID) (result bool, err error) {
	status, httpError := commands.GetPowerStatus(unitID)
	for httpError != nil {
		status, httpError = commands.GetPowerStatus(unitID)
		time.Sleep(1 * time.Second)
	}

	logger.Info().Msgf("Last Start Reason: %s", status.Spm.LastTrigger.Up)
	if status.Spm.LastTrigger.Up == "plug" {

		result = true
		return
	}
	result = false
	return
}
