package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"time"

	"github.com/DIMO-Network/edge-network/internal/gateways"
	"github.com/DIMO-Network/edge-network/internal/queue"
	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/agent"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/DIMO-Network/edge-network/service"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
	"github.com/muka/go-bluetooth/hw/linux/btmgmt"
	"github.com/muka/go-bluetooth/hw/linux/cmd"

	"github.com/sirupsen/logrus"

	"github.com/muka/go-bluetooth/hw"
)

var Version = "development"

const bleUnsupportedHW = "5.2"

const (
	adapterID = "hci0"

	appUUIDSuffix = "-6859-4d6c-a87b-8d2c98c9f6f0"
	appUUIDPrefix = "5c30"

	deviceServiceUUIDFragment       = "7fa4"
	vehicleServiceUUIDFragment      = "d387"
	primaryIDCharUUIDFragment       = "5a11"
	secondaryIDCharUUIDFragment     = "5a12"
	hwVersionUUIDFragment           = "5a13"
	signalStrengthUUIDFragment      = "5a14"
	wifiStatusUUIDFragment          = "5a15"
	setWifiUUIDFragment             = "5a16"
	softwareVersionUUIDFragment     = "5a18"
	bluetoothVersionUUIDFragment    = "5a19"
	sleepControlUUIDFragment        = "5a20"
	imsiUUIDFragment                = "5a21"
	vinCharUUIDFragment             = "0acc"
	diagCodeCharUUIDFragment        = "0add"
	protocolCharUUIDFragment        = "0adc"
	transactionsServiceUUIDFragment = "aade"
	addrCharUUIDFragment            = "1dd2"
	signCharUUIDFragment            = "e60f"
)

var lastSignature []byte

var lastVIN string
var lastProtocol string
var lastDTC string
var unitID uuid.UUID
var name string

var btManager btmgmt.BtMgmt

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

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	subcommands.Register(&scanVINCmd{unitID: unitID, logger: logger}, "decode loggers")
	subcommands.Register(&buildInfoCmd{logger: logger}, "info")

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
	// temporary for us
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	hwRevision, err := commands.GetHardwareRevision(unitID)
	if err != nil {
		logger.Err(err).Msg("error getting hardware rev")
	}
	logger.Info().Msgf("hardware version found: %s", hwRevision)

	ethAddr, ethErr := commands.GetEthereumAddress(unitID)

	if ethAddr == nil {
		if ethErr != nil {
			logger.Err(ethErr).Msg("eth addr error")
		}
		logger.Fatal().Msgf("could not get ethereum address")
	}
	// OBD / CAN Loggers
	ds := network.NewDataSender(unitID, *ethAddr, logger)

	lss := loggers.NewLoggerSettingsService()
	var qs queue.StorageQueue
	useMemoryQueue := true

	if useMemoryQueue {
		qs = queue.NewMemoryStorageQueue(unitID)
	} else {
		qs = queue.NewDiskStorageQueue(unitID)
	}

	vinLogger := loggers.NewVINLogger(logger)
	pidLogger := loggers.NewPIDLogger(unitID, qs, logger)
	vehicleSignalDecodingService := gateways.NewVehicleSignalDecodingAPIService()

	fingerprintRunner := internal.NewFingerprintRunner(unitID, vinLogger, pidLogger, ds, lss, vehicleSignalDecodingService, logger)
	// fingerprint logger, runs once on start, sends VIN & protocol
	err = fingerprintRunner.Fingerprint() // this is blocking, should it be after bluetooth setup? or run in a go func
	if err != nil {
		logger.Err(err).Msg("failed to start fingerprint logger.")
	}
	vehicleTemplates := internal.NewVehicleTemplates(logger, vehicleSignalDecodingService, lss)

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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// todo way to enable/disable our own logger engine - should be base on settings by eth addr that we pull from cloud

	// Execute Worker in background.
	runnerSvc := internal.NewWorkerRunner(unitID, ethAddr, lss, pidLogger, qs, ds, logger, vehicleTemplates)
	runnerSvc.Run() // not sure if this will block always. if it does do we need to have a cancel when catch os.Interrupt, ie. stop tasks?

	sig := <-sigChan
	logger.Info().Msgf("Terminating from signal: %s", sig)
}

func setupBluetoothApplication(logger zerolog.Logger, coldBoot bool, vinLogger loggers.VINLogger, lss loggers.LoggerSettingsService) (*service.App, context.CancelFunc, context.CancelFunc) {
	opt := service.AppOptions{
		AdapterID:         adapterID,
		AgentCaps:         agent.CapDisplayYesNo,
		AgentSetAsDefault: true,
		UUIDSuffix:        appUUIDSuffix,
		UUID:              appUUIDPrefix,
		Logger:            logger,
	}

	app, err := service.NewApp(opt)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create app: %s", err)
	}

	app.SetName(name)

	// Device service
	deviceService, err := app.NewService(deviceServiceUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create device service: %s", err)
	}

	err = app.AddService(deviceService)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add device service to app: %s", err)
	}

	// Get serial number
	unitSerialChar, err := deviceService.NewChar(primaryIDCharUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create Unit ID characteristic: %s", err)
	}

	unitSerialChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	unitSerialChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving unit serial number: %s", err)
			}
		}()

		logger.Info().Msg("Got Unit SerialNumber request")

		resp = []byte(unitID.String())
		return
	})

	err = deviceService.AddChar(unitSerialChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add SerialNumber characteristic to device service: %s", err)
	}

	// Get secondary serial number
	secondSerialChar, err := deviceService.NewChar(secondaryIDCharUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create Secondary ID characteristic: %s", err)
	}

	secondSerialChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	secondSerialChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving secondary serial number: %s", err)
			}
		}()

		logger.Info().Msg("Got Unit Secondary Id request")

		deviceID, err := commands.GetDeviceID(unitID)
		if err != nil {
			return
		}

		logger.Info().Msgf("Read Secondary: %s", deviceID)

		resp = []byte(deviceID.String())
		return
	})

	err = deviceService.AddChar(secondSerialChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add SerialNumber characteristic to device service: %s", err)
	}

	// Hardware revision
	hwRevisionChar, err := deviceService.NewChar(hwVersionUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create Hardwware Revision characteristic: %s", err)
	}

	hwRevisionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	hwRevisionChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving hardware revision: %s", err)
			}
		}()

		logger.Info().Msg("Got Hardware Revison request")

		hwRevision, err := commands.GetHardwareRevision(unitID)
		if err != nil {
			return
		}

		logger.Info().Msgf("Read Hw Revision: %s", hwRevision)

		resp = []byte(hwRevision)
		return
	})

	err = deviceService.AddChar(hwRevisionChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add Hardware Revision characteristic to device service: %s", err)
	}

	// Bluetooth revision
	bluetoothVersionChar, err := deviceService.NewChar(bluetoothVersionUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create Hardwware Revision characteristic: %s", err)
	}

	bluetoothVersionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	bluetoothVersionChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving hardware revision: %s", err)
			}
		}()

		logger.Info().Msg("Got Bluetooth Version request")

		logger.Info().Msgf("Read Bluetooth Version: %s", Version)

		resp = []byte(Version)
		return
	})

	err = deviceService.AddChar(bluetoothVersionChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add Bluetooth Version characteristic to device service: %s", err)
	}

	// Software version
	softwareVersionChar, err := deviceService.NewChar(softwareVersionUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create Hardwware Revision characteristic: %s", err)
	}

	softwareVersionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	softwareVersionChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving software version: %s", err)
			}
		}()

		logger.Info().Msg("Got Software Revison request")

		swVersion, err := commands.GetSoftwareVersion(unitID)
		if err != nil {
			return
		}

		logger.Info().Msgf("Read Software version Revision: %s", swVersion)

		resp = []byte(swVersion)
		return
	})

	err = deviceService.AddChar(softwareVersionChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add Software Version characteristic to device service: %s", err)
	}

	// Get signal strength
	signalStrengthChar, err := deviceService.NewChar(signalStrengthUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create Signal Strength characteristic: %s", err)
	}

	signalStrengthChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	signalStrengthChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving signal strength: %s", err)
			}
		}()

		logger.Info().Msg("Got Signal Strength request.")

		sigStrength, err := commands.GetSignalStrength(unitID)
		if err != nil {
			return
		}

		logger.Info().Msgf("Read Signal Strength: %s", sigStrength)

		resp = []byte(sigStrength)
		return
	})

	err = deviceService.AddChar(signalStrengthChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add Signal Strength characteristic to device service: %s", err)
	}

	// Get wifi connection status
	wifiStatusChar, err := deviceService.NewChar(wifiStatusUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create Wifi Connection Status characteristic: %s", err)
	}

	wifiStatusChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	wifiStatusChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving wifi connection status: %s", err)
			}
		}()

		logger.Info().Msg("Got Wifi Connection Status request.")

		wifiConnectionState, err := commands.GetWifiStatus(unitID)
		if err != nil {
			return
		}

		logger.Info().Msgf("Read Wifi Status: %s", wifiConnectionState)

		res := ""
		if wifiConnectionState.WPAState == "COMPLETED" {
			res = wifiConnectionState.SSID
		}

		resp = []byte(res)
		return
	})

	err = deviceService.AddChar(wifiStatusChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add Get Wifi Status characteristic to device service: %s", err)
	}

	// set wi-fi connection
	setWifiChar, err := deviceService.NewChar(setWifiUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create set wifi characteristic: %s", err)
	}

	setWifiChar.Properties.Flags = []string{
		gatt.FlagCharacteristicEncryptAuthenticatedWrite,
	}

	setWifiChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error setting wifi connection: %s.", err)
			}
		}()

		var req api.SetWifiRequest
		err = json.Unmarshal(value, &req)
		if err != nil {
			logger.Info().Msgf("Error unmarshaling wi-fi payload: %s", err)
			return
		}

		if req.Network == "" || req.Password == "" {
			logger.Info().Msgf("Missing network or password in wi-fi pairing request.")
			err = fmt.Errorf("missing network or password")
			return
		}

		newWifiList := []api.WifiEntity{
			{
				Priority: 1,
				SSID:     req.Network,
				Psk:      req.Password,
			},
		}

		setWifiResp, err := commands.SetWifiConnection(unitID, newWifiList)
		if err != nil {
			logger.Info().Msgf("Failed to set wifi connection: %s", err)
			return
		}

		if setWifiResp.Result {
			logger.Info().Msgf("Wifi Connection set successfully: %s", req.Network)
		} else {
			logger.Error().Msgf("Failed to set wifi connection: %s", err)
			return
		}

		resp = []byte(req.Network)
		return
	})

	err = deviceService.AddChar(setWifiChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add Set Wifi characteristic to device service: %s", err)
	}

	// Get IMSI
	imsiChar, err := deviceService.NewChar(imsiUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create IMSI characteristic: %s", err)
	}

	imsiChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	imsiChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving IMSI: %s", err)
			}
		}()

		logger.Info().Msg("Got IMSI request")

		imsi, err := commands.GetIMSI(unitID)
		if err != nil {
			return
		}

		resp = []byte(imsi)
		return
	})

	if err := deviceService.AddChar(imsiChar); err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add IMSI characteristic to device service: %s", err)
	}

	// Vehicle service
	vehicleService, err := app.NewService(vehicleServiceUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create vehicle service: %s", err)
	}

	err = app.AddService(vehicleService)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add vehicle service to app: %s", err)
	}

	// Get VIN
	vinChar, err := vehicleService.NewChar(vinCharUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create VIN characteristic: %s", err)
	}

	vinChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	vinChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving VIN: %s", err)
			}
		}()

		if lastVIN != "" {
			resp = []byte(lastVIN)
			logger.Info().Msgf("Returning cached VIN: %s", lastVIN)
			return
		}

		vinResp, err := vinLogger.GetVIN(unitID, nil)
		if err != nil {
			err = nil
			logger.Err(err).Msgf("Unable to get VIN")
			resp = []byte("00000000000000000")
			return
		}

		logger.Info().Msgf("Got Protocol: %s", vinResp.Protocol) // need to do something with protocol to set right template
		logger.Info().Msgf("Got VIN: %s", vinResp.VIN)
		lastVIN = vinResp.VIN
		lastProtocol = vinResp.Protocol
		resp = []byte(lastVIN)
		// we want to do this each time in case the device is being paired to a different vehicle
		err = lss.WriteVINConfig(loggers.VINLoggerSettings{VINQueryName: vinResp.QueryName})
		if err != nil {
			logger.Err(err).Msgf("failed to save vin query name in settings: %s", err)
		}
		return
	})

	err = vehicleService.AddChar(vinChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add VIN characteristic to vehicle service: %s", err)
	}

	// Get Protocol (based on what query worked to get the VIN, must Get VIN before)
	protocolChar, err := vehicleService.NewChar(protocolCharUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create protocol characteristic: %s", err)
	}
	protocolChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	protocolChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving Protocol: %s", err)
			}
		}()
		if lastProtocol != "" {
			resp = []byte(lastProtocol)
			logger.Info().Msgf("Returning protocol from last VIN query: %s", lastProtocol)
			return
		}
		// just re-query for VIN
		vinResp, err := vinLogger.GetVIN(unitID, nil)
		if err != nil {
			err = nil
			logger.Err(err).Msgf("Unable to get VIN")
			resp = []byte("00")
			return
		}

		logger.Info().Msgf("Got Protocol: %s", vinResp.Protocol)
		lastVIN = vinResp.VIN
		lastProtocol = vinResp.Protocol
		resp = []byte(lastProtocol)
		return
	})
	err = vehicleService.AddChar(protocolChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add protocol characteristic to vehicle service: %s", err)
	}

	// Diagnostic codes
	dtcChar, err := vehicleService.NewChar(diagCodeCharUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create diagnostic Code characteristic: %s", err)
	}

	dtcChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead, gatt.FlagCharacteristicEncryptAuthenticatedWrite}

	// dtcChar will return error codes if found, if nothing found with a success will return "0", if nothing found but error response returns "1"
	dtcChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error retrieving diagnostic codes: %s", err)
			}
		}()
		if lastDTC != "" {
			resp = []byte(lastDTC)
			logger.Info().Msgf("Returning DTC codes from last DTC query over bluetooth: %s", lastDTC)
			return
		}
		logger.Info().Msg("Got diagnostic request")

		codes, err := commands.GetDiagnosticCodes(unitID, logger)
		if err != nil {
			resp = []byte("1")
			return
		}
		logger.Info().Msgf("Got Error Codes: %s", codes)

		if len(codes) < 2 {
			codes = "0"
		}

		resp = []byte(codes)
		lastDTC = codes
		return
	})

	dtcChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error clearing diagnostic codes hash: %s.", err)
			}
		}()

		logger.Info().Msgf("Got clear DTC request")

		err = commands.ClearDiagnosticCodes(unitID)
		if err != nil {
			return
		}

		logger.Info().Msgf("Cleared DTCs")

		return
	})

	err = vehicleService.AddChar(dtcChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add diagnostic characteristic to vehicle service: %s", err)
	}

	// Sleep Control
	sleepControlChar, err := deviceService.NewChar(sleepControlUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create sleep control characteristic: %s", err)
	}

	sleepControlChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedWrite}

	sleepControlChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error extending sleep time: %s.", err)
			}
		}()

		logger.Info().Msgf("Got extend sleep request")

		err = commands.ExtendSleepTimer(unitID)
		if err != nil {
			return
		}

		logger.Info().Msgf("Extended sleep time to 900 seconds")

		return
	})

	err = deviceService.AddChar(sleepControlChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add sleep control characteristic to device service: %s", err)
	}

	// Transactions service
	transactionsService, err := app.NewService(transactionsServiceUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create transaction service: %s", err)
	}

	err = app.AddService(transactionsService)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add transaction service to app: %s", err)
	}

	// Get Ethereum address
	addrChar, err := transactionsService.NewChar(addrCharUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create get ethereum address characteristic: %s", err)
	}

	addrChar.Properties.Flags = []string{
		gatt.FlagCharacteristicRead,
	}

	addrChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		logger.Info().Msg("Got address request")

		addr, err := commands.GetEthereumAddress(unitID)
		if err != nil {
			return
		}

		resp = addr[:]

		return
	})

	err = transactionsService.AddChar(addrChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add Ethereum address characteristic: %s", err)
	}

	// Sign hash
	signChar, err := transactionsService.NewChar(signCharUUIDFragment)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to create sign hash characteristic: %s", err)
	}

	signChar.Properties.Flags = []string{
		gatt.FlagCharacteristicEncryptAuthenticatedWrite,
		gatt.FlagCharacteristicEncryptAuthenticatedRead,
	}

	signChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				logger.Err(err).Msgf("Error signing hash: %s.", err)
			}
		}()

		// Wipe any old value so that if this fails, the client doesn't mistakenly
		// think everything is fine.
		lastSignature = nil

		if l := len(value); l != 32 {
			err = fmt.Errorf("input has byte length %d, must be 32", l)
			return
		}

		logger.Info().Msgf("Got sign request for hash: %s.", hex.EncodeToString(value))

		sig, err := commands.SignHash(unitID, value)
		if err != nil {
			return
		}

		lastSignature = sig

		logger.Info().Msgf("Signature: %s.", hex.EncodeToString(sig))

		return
	})

	signChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		logger.Info().Msgf("Got read request for hash: %s.", hex.EncodeToString(lastSignature))
		resp = lastSignature
		return
	})

	err = transactionsService.AddChar(signChar)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to add hash signing characteristic: %s", err)
	}

	err = app.Run()
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to initialize app: %s", err)
	}

	//Check if we should disable new connections
	devices, err := app.Adapter().GetDevices()
	if err != nil {
		logger.Fatal().Err(err).Msgf("Could not retrieve previously paired devices: %s", err)
	}

	if !coldBoot && hasPairedDevices(logger, devices) {
		logger.Info().Msgf("Disabling bonding")
		err = btManager.SetBondable(false)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to set bonding status: %s", err)
		}
	}

	adapterName, err := app.Adapter().GetName()
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to get adapter name: %s", err)
	}

	adapterAlias, err := app.Adapter().GetAlias()
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to get adapter alias: %s", err)
	}

	canBusInformation, err := commands.DetectCanbus(unitID)
	if err != nil {
		logger.Err(err).Msgf("Failed to autodetect a canbus: %s", err)
	}

	advertisedServices := []string{app.GenerateUUID(deviceServiceUUIDFragment)}

	cancel, err := app.Advertise(math.MaxUint32, name, advertisedServices)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed advertising: %s", err)
	}

	omSignal, omSignalCancel, err := app.Adapter().GetObjectManagerSignal()
	if err != nil {
		logger.Fatal().Err(err).Msgf("Failed to Get Signal")
	}

	go func() {
		// Recover from panic on errors in the loop
		defer func() {
			if err := recover(); err != nil {
				logger.Error().Msgf("Recovering from panic: %s", err)
			}
		}()

		for v := range omSignal {

			if v == nil {
				return
			}
			if v.Name == bluez.InterfacesRemoved {
				// re-enables advertising, bug in the driver
				_, err := cmd.Exec("hciconfig", adapterID, "leadv 0")
				if err != nil {
					logger.Err(err).Msgf("error executing hciconfig: %s", err)
				}
			} else {
				continue
			}
		}
	}()

	logger.Info().Msgf("Canbus Protocol Info: %v", canBusInformation)
	logger.Info().Msgf("Adapter address: %s", app.Adapter().Properties.Address)
	logger.Info().Msgf("Adapter name: %s, alias: %s", adapterName, adapterAlias)

	logger.Info().Msgf("Device service: %s", deviceService.Properties.UUID)
	logger.Info().Msgf("  Get Serial Number characteristic: %s", unitSerialChar.Properties.UUID)
	logger.Info().Msgf("  Get Secondary ID characteristic: %s", secondSerialChar.Properties.UUID)
	logger.Info().Msgf("  Get Hardware Revision characteristic: %s", hwRevisionChar.Properties.UUID)
	logger.Info().Msgf("  Get Software Version characteristic: %s", softwareVersionChar.Properties.UUID)
	logger.Info().Msgf("  Set Bluetooth Version characteristic: %s", bluetoothVersionChar.Properties.UUID)
	logger.Info().Msgf("  Sleep Control characteristic: %s", sleepControlChar.Properties.UUID)
	logger.Info().Msgf("  Get Signal Strength characteristic: %s", signalStrengthChar.Properties.UUID)
	logger.Info().Msgf("  Get Wifi Connection Status characteristic: %s", wifiStatusChar.Properties.UUID)
	logger.Info().Msgf("  Set Wifi Connection characteristic: %s", setWifiChar.Properties.UUID)
	logger.Info().Msgf("  Get IMSI characteristic: %s", imsiChar.Properties.UUID)

	logger.Info().Msgf("Vehicle service: %s", vehicleService.Properties.UUID)
	logger.Info().Msgf("  Get VIN characteristic: %s", vinChar.Properties.UUID)
	logger.Info().Msgf("  Get DTC characteristic: %s", dtcChar.Properties.UUID)
	logger.Info().Msgf("  Clear DTC characteristic: %s", dtcChar.Properties.UUID)

	logger.Info().Msgf("Transactions service: %s", transactionsService.Properties.UUID)
	logger.Info().Msgf("  Get ethereum address characteristic: %s", addrChar.Properties.UUID)
	logger.Info().Msgf("  Sign hash characteristic: %s", signChar.Properties.UUID)

	return app, cancel, omSignalCancel
}

// Utility Function
func hasPairedDevices(logger zerolog.Logger, devices []*device.Device1) bool {
	for _, d := range devices {
		logger.Info().Msgf("Found previously connected device: %v", d.Properties.Alias)
		if d.Properties.Trusted && d.Properties.Paired {
			return true
		}
	}
	return false
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
