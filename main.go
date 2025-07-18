package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/DIMO-Network/edge-network/internal/hooks"
	"github.com/DIMO-Network/edge-network/internal/util/retry"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"

	"github.com/DIMO-Network/edge-network/certificate"
	dimoConfig "github.com/DIMO-Network/edge-network/config"
	"github.com/DIMO-Network/edge-network/internal/gateways"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/hw"
	"github.com/muka/go-bluetooth/hw/linux/btmgmt"
)

var Version = "development"
var ENV = "prod"

const bleUnsupportedHW = "5.2"

var unitID uuid.UUID
var name string

var btManager btmgmt.BtMgmt

//go:embed config.yaml config-dev.yaml
var configFiles embed.FS

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
	// Used by go-bluetooth, and we use this to set how much it logs. Not for this project.
	logrus.SetLevel(logrus.InfoLevel)

	name, unitID = commands.GetDeviceName(logger)
	logger.Info().Msgf("SerialNumber Number: %s", unitID)

	hwRevision, err := commands.GetHardwareRevision(unitID)
	if err != nil {
		logger.Err(err).Msg("error getting hardware rev")
	}
	logger.Info().Msgf("hardware version found: %s", hwRevision)

	// retry logic for getting ethereum address
	ethAddr, ethErr := retry.Retry[common.Address](3, 5*time.Second, logger, func() (interface{}, error) {
		return commands.GetEthereumAddress(unitID)
	})

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	subcommands.Register(&scanVINCmd{unitID: unitID, logger: logger}, "decode loggers")
	subcommands.Register(&buildInfoCmd{logger: logger}, "info")
	subcommands.Register(&canDumpCmd{unitID: unitID}, "canDump operations")
	subcommands.Register(&dbcScanCmd{logger: logger}, "decode loggers")
	subcommands.Register(&canDumpV2Cmd{logger: logger}, "decode loggers")

	if len(os.Args) > 1 {
		ctx := context.Background()
		flag.Parse()
		os.Exit(int(subcommands.Execute(ctx)))
	}

	// define environment
	var env gateways.Environment
	var confFileName string
	var configURL string
	if ENV == "prod" {
		env = gateways.Production
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		confFileName = "config.yaml"
		configURL = "https://device-config.dimo.xyz"
	} else {
		env = gateways.Development
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		confFileName = "config-dev.yaml"
		configURL = "https://device-config-dev.dimo.xyz"
	}

	lss := loggers.NewTemplateStore()
	vinLogger := loggers.NewVINLogger(logger)

	logger.Info().Msgf("Bluetooth name: %s", name)
	logger.Info().Msgf("Version: %s", Version)
	logger.Info().Msgf("Environment: %s", env)

	coldBoot, err := isColdBoot(logger, unitID)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to get power management status: %s", err)
	}
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

	// read config file
	// will retry for about 1 hour in case if no internet connection, so we are not interrupt device pairing process
	config, confErr := dimoConfig.ReadConfig(logger, configFiles, configURL, confFileName)
	logger.Debug().Msgf("Config: %+v\n", config)
	if confErr != nil {
		logger.Fatal().Err(confErr).Msg("unable to read config file")
	}

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
	cs := certificate.NewCertificateService(logger, *config, nil, certificate.CertFileWriter{})
	certErr := cs.CheckCertAndRenewIfExpiresSoon(*ethAddr, unitID)

	// setup datasender here so we can send errors to it
	ds := network.NewDataSender(unitID, *ethAddr, logger, models.VehicleInfo{}, *config)
	//  From this point forward, any log events produced by this logger will pass through the hook.
	fh := hooks.NewLogRateLimiterHook(ds)
	logger = logger.Hook(&hooks.LogHook{DataSender: ds}).Hook(fh)

	// log certificate errors
	if certErr != nil {
		logger.Error().Ctx(context.WithValue(context.Background(), hooks.LogToMqtt, "true")).Msgf("Error from SignWeb3Certificate : %s", certErr.Error())
	}

	// block here until satisfy condition. future - way to know if device is being used as decoding device, eg. mapped to a specific template
	// and we want to loosen some assumptions, eg. doesn't matter if not paired.
	vehicleInfo, err := blockingGetVehicleInfo(logger, ethAddr, lss, *config)
	if err != nil {
		logger.Fatal().Err(err).Msgf("cannot start edge-network because no on-chain pairing was found for this device addr: %s", ethAddr.Hex())
	}

	// OBD / CAN Loggers
	// set vehicle info here, so we can use it for status messages
	ds.SetVehicleInfo(*vehicleInfo)
	vehicleSignalDecodingAPI := gateways.NewVehicleSignalDecodingAPIService(*config)
	vehicleTemplates := internal.NewVehicleTemplates(logger, vehicleSignalDecodingAPI, lss)

	// get the template settings from remote, below method handles all the special logic
	pids, deviceSettings, dbcFile, err := vehicleTemplates.GetTemplateSettings(ethAddr, Version, unitID)
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
	if dbcFile != nil {
		logger.Info().Msgf("found dbc file: %s", *dbcFile)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	fingerprintRunner := internal.NewFingerprintRunner(unitID, vinLogger, ds, lss, logger)
	dtcRunner := internal.NewDtcErrorsRunner(unitID, ds, logger)
	dbcScanner := loggers.NewDBCPassiveLogger(logger, dbcFile, hwRevision, pids)

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
	runnerSvc := internal.NewWorkerRunner(ethAddr, lss, ds, logger, fingerprintRunner, pids, deviceSettings, deviceConf, vehicleInfo, dbcScanner, dtcRunner)
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
func getVehicleInfo(logger zerolog.Logger, ethAddr *common.Address, conf dimoConfig.Config) (*models.VehicleInfo, error) {
	identityAPIService := gateways.NewIdentityAPIService(logger, conf)
	vehicleDefinition, err := retry.Retry[models.VehicleInfo](3, 1*time.Second, logger, func() (interface{}, error) {
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
func blockingGetVehicleInfo(logger zerolog.Logger, ethAddr *common.Address, lss loggers.TemplateStore, conf dimoConfig.Config) (*models.VehicleInfo, error) {
	for i := 0; i < 60; i++ {
		vehicleInfo, err := getVehicleInfo(logger, ethAddr, conf)
		if err != nil {
			logger.Err(err).Ctx(context.WithValue(context.Background(), hooks.LogToMqtt, "true")).Msgf("failed to get vehicle info, will retry again in 60s")
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
