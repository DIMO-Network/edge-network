package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/DIMO-Network/edge-network/certificate"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/ethereum/go-ethereum/common"
	"os"
	"os/signal"
	"strings"
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
var ENV = "prod"

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
	// Used by go-bluetooth, and we use this to set how much it logs. Not for this project.
	logrus.SetLevel(logrus.InfoLevel)

	// define environment
	var env gateways.Environment
	if ENV == "prod" {
		env = gateways.Production
	} else {
		env = gateways.Development
	}
	// temporary for us, for release want info level - todo make configurable via cli?
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	logger.Info().Msgf("Starting DIMO Edge Network, with log level: %s", zerolog.GlobalLevel())

	// check if we were able to get ethereum address, otherwise fail fast
	if ethAddr == nil {
		if ethErr != nil {
			logger.Err(ethErr).Msg("eth addr error")
		}
		logger.Fatal().Msgf("could not get ethereum address")
	} else {
		logger.Info().Msgf("Device Ethereum Address: %s", ethAddr.Hex())
	}

	//  start mqtt certificate verification routine
	cs := certificate.NewCertificateService(logger, env, nil, certificate.CertFileWriter{})
	err := cs.CheckCertAndRenewIfExpiresSoon(*ethAddr, unitID)

	if err != nil {
		logger.Err(err).Msgf("Error from SignWeb3Certificate : %v", err)
	}

	// setup datasender here so we can send errors to it
	ds := network.NewDataSender(unitID, *ethAddr, logger, nil, true)
	//  From this point forward, any log events produced by this logger will pass through the hook.
	logger = logger.Hook(&internal.LogHook{DataSender: ds})

	coldBoot, err := isColdBoot(logger, unitID)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to get power management status: %s", err)
	}

	logger.Info().Msgf("Bluetooth name: %s", name)
	logger.Info().Msgf("Version: %s", Version)
	logger.Info().Msgf("Environment: %s", env)

	hwRevision, err := commands.GetHardwareRevision(unitID)
	if err != nil {
		logger.Err(err).Msg("error getting hardware rev")
	}
	logger.Info().Msgf("hardware version found: %s", hwRevision)

	lss := loggers.NewTemplateStore()
	vinLogger := loggers.NewVINLogger(logger)
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

	// block here until satisfy condition. future - way to know if device is being used as decoding device, eg. mapped to a specific template
	// and we want to loosen some assumptions, eg. doesn't matter if not paired.
	vehicleInfo, err := blockingGetVehicleInfo(logger, ethAddr, lss, env)
	if err != nil {
		logger.Fatal().Err(err).Msgf("cannot start edge-network because no on-chain pairing was found for this device addr: %s", ethAddr.Hex())
	}

	// OBD / CAN Loggers
	// set vehicle info here, so we can use it for status messages
	ds.SetVehicleInfo(vehicleInfo)
	vehicleSignalDecodingAPI := gateways.NewVehicleSignalDecodingAPIService(env)
	vehicleTemplates := internal.NewVehicleTemplates(logger, vehicleSignalDecodingAPI, lss)

	// get the template settings from remote, below method handles all the special logic
	pids, deviceSettings, err := vehicleTemplates.GetTemplateSettings(ethAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to get device settings (pids, dbc, settings)")
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

// getVehicleInfo queries identity-api with 3 retries logic, to get vehicle to device pairing info (vehicle NFT)
func getVehicleInfo(logger zerolog.Logger, ethAddr *common.Address, env gateways.Environment) (*models.VehicleInfo, error) {
	identityAPIService := gateways.NewIdentityAPIService(logger, env)
	vehicleDefinition, err := gateways.Retry[models.VehicleInfo](3, 1*time.Second, logger, func() (interface{}, error) {
		v, err := identityAPIService.QueryIdentityAPIForVehicle(*ethAddr)
		if v != nil && v.TokenID == 0 {
			return nil, fmt.Errorf("failed to query identity api for vehicle info - tokenId is zero")
		}
		return v, err
	})
	if err != nil {
		return nil, err
	}

	return vehicleDefinition, nil
}

// blockingGetVehicleInfo is a function that retries getVehicleInfo for 60 times with a 60-second interval.
// If the vehicle info is retrieved successfully, it is written to a temporary cache. If the error is not tokenId zero,
// which would mean no pairing, then check the local cache since this is likely transient error.
// If the vehicle info is not retrieved within the retries, a timeout error is returned.
func blockingGetVehicleInfo(logger zerolog.Logger, ethAddr *common.Address, lss loggers.TemplateStore, env gateways.Environment) (*models.VehicleInfo, error) {
	for i := 0; i < 60; i++ {
		vehicleInfo, err := getVehicleInfo(logger, ethAddr, env)
		if err != nil {
			// todo future: send each err failure to logs mqtt topic
			logger.Err(err).Msgf("failed to get vehicle info, will retry again in 60s")
			// check for local cache but only if error is not of type tokenid zero
			if !strings.Contains(err.Error(), "tokenId is zero") {
				vehicleInfo, err = lss.ReadVehicleInfo()
				if err != nil && vehicleInfo != nil {
					return vehicleInfo, err
				}
			}
		}
		if vehicleInfo != nil {
			logger.Info().Msgf("identity-api vehicle info: %+v", vehicleInfo)
			vehInfoErr := lss.WriteVehicleInfo(*vehicleInfo)
			if vehInfoErr != nil {
				logger.Err(vehInfoErr).Msg("error writing vehicle info to tmp cache")
			}
			return vehicleInfo, nil
		}
		logger.Error().Msg("unable to get vehicle info from identity-api, trying again in 60s")
		time.Sleep(60 * time.Second)
	}
	return nil, fmt.Errorf("timed out waiting for identity-api vehicle info after many retries")
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
