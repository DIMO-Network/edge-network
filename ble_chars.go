package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	"github.com/DIMO-Network/edge-network/internal/hooks"

	"github.com/DIMO-Network/edge-network/agent"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/edge-network/service"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
	"github.com/muka/go-bluetooth/hw/linux/cmd"
	"github.com/rs/zerolog"
)

// define the BLE characteristic codes under which they'll be discoverable
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

// variables used to hold data read from BLE
var lastVIN string
var lastSignature []byte
var lastProtocol string
var lastDTC string

func setupBluetoothApplication(logger zerolog.Logger, coldBoot bool, vinLogger loggers.VINLogger, lss loggers.SettingsStore) (*service.App, context.CancelFunc, context.CancelFunc) {
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
		hooks.LogFatal(logger, err, "failed to create app")
	}

	app.SetName(name)

	// Device service
	deviceService, err := app.NewService(deviceServiceUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create device service")
	}

	err = app.AddService(deviceService)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to add device service to app")
	}

	// Get serial number
	unitSerialChar, err := deviceService.NewChar(primaryIDCharUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create unit ID characteristic")
	}

	unitSerialChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	unitSerialChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add SerialNumber characteristic to device service")
	}

	// Get secondary serial number
	secondSerialChar, err := deviceService.NewChar(secondaryIDCharUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create Secondary ID characteristic")
	}

	secondSerialChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	secondSerialChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Secondary ID characteristic to device service")
	}

	// Hardware revision
	hwRevisionChar, err := deviceService.NewChar(hwVersionUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create Hardware Revision characteristic")
	}

	hwRevisionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	hwRevisionChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Hardware Revision characteristic to device service")
	}

	// Bluetooth revision
	bluetoothVersionChar, err := deviceService.NewChar(bluetoothVersionUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create Hardware Revision characteristic")
	}

	bluetoothVersionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	bluetoothVersionChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Bluetooth Version characteristic to device service")
	}

	// Software version
	softwareVersionChar, err := deviceService.NewChar(softwareVersionUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create Hardware Revision characteristic")
	}

	softwareVersionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	softwareVersionChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Software Version characteristic to device service")
	}

	// Get signal strength
	signalStrengthChar, err := deviceService.NewChar(signalStrengthUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create Signal Strength characteristic")
	}

	signalStrengthChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	signalStrengthChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Signal Strength characteristic to device service")
	}

	// Get wifi connection status
	wifiStatusChar, err := deviceService.NewChar(wifiStatusUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create Wifi Connection Status characteristic")
	}

	wifiStatusChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	wifiStatusChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Wifi Connection Status characteristic to device service")
	}

	// set wi-fi connection
	setWifiChar, err := deviceService.NewChar(setWifiUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create Set Wifi characteristic")
	}

	setWifiChar.Properties.Flags = []string{
		gatt.FlagCharacteristicEncryptAuthenticatedWrite,
	}

	setWifiChar.OnWrite(func(_ *service.Char, value []byte) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Set Wifi characteristic to device service")
	}

	// Get IMSI
	imsiChar, err := deviceService.NewChar(imsiUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create IMSI characteristic")
	}

	imsiChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	imsiChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add IMSI characteristic to device service")
	}

	// Vehicle service
	vehicleService, err := app.NewService(vehicleServiceUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create vehicle service")
	}

	err = app.AddService(vehicleService)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to add vehicle service to app")
	}

	// Get VIN
	vinChar, err := vehicleService.NewChar(vinCharUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create VIN characteristic")
	}

	vinChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	// normally gets called during device pairing from mobile App
	vinChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				hooks.LogError(logger, err, "error retrieving VIN via BLE", hooks.WithThresholdWhenLogMqtt(1))
			}
		}()

		if lastVIN != "" {
			resp = []byte(lastVIN)
			logger.Info().Msgf("Returning cached VIN: %s", lastVIN)
			return
		}
		// clear all settings since this is most likely a brand new pairing
		errDel := lss.DeleteAllSettings()
		if errDel != nil {
			logger.Err(errDel).Msgf("there was one or more errors deleting settings from disk, continuing")
		}

		vinResp, err := vinLogger.GetVIN(unitID, nil)
		if err != nil {
			logger.Err(err).Msgf("Unable to get VIN")
			resp = []byte("00000000000000000")
			return
		}

		logger.Info().Msgf("Got Protocol: %s", vinResp.Protocol) // verify using protocol when requesting template
		hooks.LogInfo(logger, "Got VIN via BLE", hooks.WithThresholdWhenLogMqtt(1))
		logger.Info().Msg(vinResp.VIN)
		lastVIN = vinResp.VIN
		lastProtocol = vinResp.Protocol
		resp = []byte(lastVIN)
		// we want to do this each time in case the device is being paired to a different vehicle
		errSaveCfg := lss.WriteVINConfig(models.VINLoggerSettings{VINQueryName: vinResp.QueryName, VIN: lastVIN})
		if errSaveCfg != nil {
			hooks.LogError(logger, errSaveCfg, "failed to save vin query name in settings", hooks.WithThresholdWhenLogMqtt(1))
		}
		// todo restart the application?
		return
	})

	err = vehicleService.AddChar(vinChar)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to add VIN characteristic to vehicle service")
	}

	// Get Protocol (based on what query worked to get the VIN, must Get VIN before)
	protocolChar, err := vehicleService.NewChar(protocolCharUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create protocol characteristic")
	}
	protocolChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	protocolChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				hooks.LogError(logger, err, "error retrieving protocol via BLE", hooks.WithThresholdWhenLogMqtt(1))
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
		hooks.LogFatal(logger, err, "failed to add protocol characteristic to vehicle service")
	}

	// Diagnostic codes
	dtcChar, err := vehicleService.NewChar(diagCodeCharUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create diagnostic code characteristic")
	}

	dtcChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead, gatt.FlagCharacteristicEncryptAuthenticatedWrite}

	// dtcChar will return error codes if found, if nothing found with a success will return "0", if nothing found but error response returns "1"
	dtcChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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

	dtcChar.OnWrite(func(_ *service.Char, _ []byte) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add diagnostic characteristic to vehicle service")
	}

	// Sleep Control
	sleepControlChar, err := deviceService.NewChar(sleepControlUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create sleep control characteristic")
	}

	sleepControlChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedWrite}

	sleepControlChar.OnWrite(func(_ *service.Char, _ []byte) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add sleep control characteristic to device service")
	}

	// Transactions service
	transactionsService, err := app.NewService(transactionsServiceUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create transaction service")
	}

	err = app.AddService(transactionsService)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to add transaction service to app")
	}

	// Get Ethereum address
	addrChar, err := transactionsService.NewChar(addrCharUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create get ethereum address characteristic")
	}

	addrChar.Properties.Flags = []string{
		gatt.FlagCharacteristicRead,
	}

	addrChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
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
		hooks.LogFatal(logger, err, "failed to add Ethereum address characteristic")
	}

	// Sign hash
	signChar, err := transactionsService.NewChar(signCharUUIDFragment)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to create sign hash characteristic")
	}

	signChar.Properties.Flags = []string{
		gatt.FlagCharacteristicEncryptAuthenticatedWrite,
		gatt.FlagCharacteristicEncryptAuthenticatedRead,
	}

	signChar.OnWrite(func(_ *service.Char, value []byte) (resp []byte, err error) {
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

	signChar.OnRead(func(_ *service.Char, _ map[string]interface{}) (resp []byte, err error) {
		logger.Info().Msgf("Got read request for hash: %s.", hex.EncodeToString(lastSignature))
		resp = lastSignature
		return
	})

	err = transactionsService.AddChar(signChar)
	if err != nil {
		hooks.LogFatal(logger, err, "failed to add hash signing characteristic")
	}

	err = app.Run()
	if err != nil {
		hooks.LogFatal(logger, err, "failed to initialize app")
	}

	//Check if we should disable new connections
	devices, err := app.Adapter().GetDevices()
	if err != nil {
		hooks.LogFatal(logger, err, "could not retrieve previously paired devices")
	}

	if !coldBoot && hasPairedDevices(logger, devices) {
		logger.Info().Msgf("Disabling bonding")
		err = btManager.SetBondable(false)
		if err != nil {
			hooks.LogFatal(logger, err, "failed to set bonding status")
		}
	}

	adapterName, err := app.Adapter().GetName()
	if err != nil {
		hooks.LogFatal(logger, err, "failed to get adapter name")
	}

	adapterAlias, err := app.Adapter().GetAlias()
	if err != nil {
		hooks.LogFatal(logger, err, "failed to get adapter alias")
	}

	canBusInformation, err := commands.DetectCanbus(unitID)
	if err != nil {
		logger.Err(err).Msgf("Failed to autodetect a canbus: %s", err)
	}

	advertisedServices := []string{app.GenerateUUID(deviceServiceUUIDFragment)}

	cancel, err := app.Advertise(math.MaxUint32, name, advertisedServices)
	if err != nil {
		hooks.LogFatal(logger, err, "failed advertising")
	}

	omSignal, omSignalCancel, err := app.Adapter().GetObjectManagerSignal()
	if err != nil {
		hooks.LogFatal(logger, err, "failed to get signal")
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
	logger.Debug().Msgf("Adapter address: %s", app.Adapter().Properties.Address)
	logger.Debug().Msgf("Adapter name: %s, alias: %s", adapterName, adapterAlias)

	logger.Debug().Msgf("Device service: %s", deviceService.Properties.UUID)
	logger.Debug().Msgf("  Get Serial Number characteristic: %s", unitSerialChar.Properties.UUID)
	logger.Debug().Msgf("  Get Secondary ID characteristic: %s", secondSerialChar.Properties.UUID)
	logger.Debug().Msgf("  Get Hardware Revision characteristic: %s", hwRevisionChar.Properties.UUID)
	logger.Debug().Msgf("  Get Software Version characteristic: %s", softwareVersionChar.Properties.UUID)
	logger.Debug().Msgf("  Set Bluetooth Version characteristic: %s", bluetoothVersionChar.Properties.UUID)
	logger.Debug().Msgf("  Sleep Control characteristic: %s", sleepControlChar.Properties.UUID)
	logger.Debug().Msgf("  Get Signal Strength characteristic: %s", signalStrengthChar.Properties.UUID)
	logger.Debug().Msgf("  Get Wifi Connection Status characteristic: %s", wifiStatusChar.Properties.UUID)
	logger.Debug().Msgf("  Set Wifi Connection characteristic: %s", setWifiChar.Properties.UUID)
	logger.Debug().Msgf("  Get IMSI characteristic: %s", imsiChar.Properties.UUID)

	logger.Debug().Msgf("Vehicle service: %s", vehicleService.Properties.UUID)
	logger.Debug().Msgf("  Get VIN characteristic: %s", vinChar.Properties.UUID)
	logger.Debug().Msgf("  Get DTC characteristic: %s", dtcChar.Properties.UUID)
	logger.Debug().Msgf("  Clear DTC characteristic: %s", dtcChar.Properties.UUID)

	logger.Debug().Msgf("Transactions service: %s", transactionsService.Properties.UUID)
	logger.Debug().Msgf("  Get ethereum address characteristic: %s", addrChar.Properties.UUID)
	logger.Debug().Msgf("  Sign hash characteristic: %s", signChar.Properties.UUID)

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
