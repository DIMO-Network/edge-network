package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"time"

	"github.com/DIMO-Network/edge-network/agent"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/service"
	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
	"github.com/muka/go-bluetooth/hw/linux/btmgmt"

	"github.com/sirupsen/logrus"

	"github.com/muka/go-bluetooth/hw"
)

var Version = "development"

const (
	adapterID = "hci0"

	appUUIDSuffix = "-6859-4d6c-a87b-8d2c98c9f6f0"
	appUUIDPrefix = "5c30"

	deviceServiceUUIDFragment       = "7fa4"
	vehicleServiceUUIDFragment      = "d387"
	primaryIdCharUUIDFragment       = "5a11"
	secondaryIdCharUUIDFragment     = "5a12"
	hwVersionUUIDFragment           = "5a13"
	signalStrengthUUIDFragment      = "5a14"
	wifiStatusUUIDFragment          = "5a15"
	setWifiUUIDFragment             = "5a16"
	softwareVersionUUIDFragment     = "5a18"
	bluetoothVersionUUIDFragment    = "5a19"
	sleepControlUUIDFragment        = "5a20"
	vinCharUUIDFragment             = "0acc"
	diagCodeCharUUIDFragment        = "0add"
	transactionsServiceUUIDFragment = "aade"
	addrCharUUIDFragment            = "1dd2"
	signCharUUIDFragment            = "e60f"
)

var lastSignature []byte

var lastVIN string
var unitId uuid.UUID
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
	if len(os.Args) > 1 {
		s := os.Args[1]
		if s == "-v" {
			log.Printf("Version: %s", Version)
			os.Exit(0)
		}
	}
	log.Printf("Starting DIMO Edge Network")

	name, unitId = commands.GetDeviceName()
	log.Printf("Serial Number: %s", unitId)

	coldBoot, err := isColdBoot(unitId)
	if err != nil {
		log.Fatalf("Failed to get power management status: %s", err)
	}
	log.Printf("Bluetooth name: %s", name)
	log.Printf("Version: %s", Version)
	log.Printf("Sleeping 7 seconds before setting up bluez.")
	time.Sleep(7 * time.Second)
	// Used by go-bluetooth.
	// TODO(elffjs): Turn this off?
	logrus.SetLevel(logrus.TraceLevel)

	err = setupBluez(name)
	if err != nil {
		log.Fatalf("Failed to setup BlueZ: %s", err)
	}

	opt := service.AppOptions{
		AdapterID:         adapterID,
		AgentCaps:         agent.CapDisplayYesNo,
		AgentSetAsDefault: true,
		UUIDSuffix:        appUUIDSuffix,
		UUID:              appUUIDPrefix,
	}

	app, err := service.NewApp(opt)
	if err != nil {
		log.Fatalf("Failed to create app: %s", err)
	}

	defer app.Close()

	app.SetName(name)

	// Device service
	deviceService, err := app.NewService(deviceServiceUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create device service: %s", err)
	}

	err = app.AddService(deviceService)
	if err != nil {
		log.Fatalf("Failed to add device service to app: %s", err)
	}

	// Get serial number
	unitSerialChar, err := deviceService.NewChar(primaryIdCharUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Unit ID characteristic: %s", err)
	}

	unitSerialChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	unitSerialChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving unit serial number: %s", err)
			}
		}()

		log.Print("Got Unit Serial request")

		resp = []byte(unitId.String())
		return
	})

	err = deviceService.AddChar(unitSerialChar)
	if err != nil {
		log.Fatalf("Failed to add UnitID characteristic to device service: %s", err)
	}

	// Get secondary serial number
	secondSerialChar, err := deviceService.NewChar(secondaryIdCharUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Secondary ID characteristic: %s", err)
	}

	secondSerialChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	secondSerialChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving secondary serial number: %s", err)
			}
		}()

		log.Print("Got Unit Secondary Id request")

		deviceID, err := commands.GetDeviceID(unitId)
		if err != nil {
			return
		}

		log.Printf("Read Secondary: %s", deviceID)

		resp = []byte(deviceID.String())
		return
	})

	err = deviceService.AddChar(secondSerialChar)
	if err != nil {
		log.Fatalf("Failed to add UnitID characteristic to device service: %s", err)
	}

	// Hardware revision
	hwRevisionChar, err := deviceService.NewChar(hwVersionUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Hardwware Revision characteristic: %s", err)
	}

	hwRevisionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	hwRevisionChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving hardware revision: %s", err)
			}
		}()

		log.Print("Got Hardware Revison request")

		hwRevision, err := commands.GetHardwareRevision(unitId)
		if err != nil {
			return
		}

		log.Printf("Read Hw Revision: %s", hwRevision)

		resp = []byte(hwRevision)
		return
	})

	err = deviceService.AddChar(hwRevisionChar)
	if err != nil {
		log.Fatalf("Failed to add Hardware Revision characteristic to device service: %s", err)
	}

	// Bluetooth revision
	bluetoothVersionChar, err := deviceService.NewChar(bluetoothVersionUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Hardwware Revision characteristic: %s", err)
	}

	bluetoothVersionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	bluetoothVersionChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving hardware revision: %s", err)
			}
		}()

		log.Print("Got Bluetooth Version request")

		log.Printf("Read Bluetooth Version: %s", Version)

		resp = []byte(Version)
		return
	})

	err = deviceService.AddChar(bluetoothVersionChar)
	if err != nil {
		log.Fatalf("Failed to add Bluetooth Version characteristic to device service: %s", err)
	}

	// Software version
	softwareVersionChar, err := deviceService.NewChar(softwareVersionUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Hardwware Revision characteristic: %s", err)
	}

	softwareVersionChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	softwareVersionChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving software version: %s", err)
			}
		}()

		log.Print("Got Software Revison request")

		swVersion, err := commands.GetSoftwareVersion(unitId)
		if err != nil {
			return
		}

		log.Printf("Read Software version Revision: %s", swVersion)

		resp = []byte(swVersion)
		return
	})

	err = deviceService.AddChar(softwareVersionChar)
	if err != nil {
		log.Fatalf("Failed to add Software Version characteristic to device service: %s", err)
	}

	// Get signal strength
	signalStrengthChar, err := deviceService.NewChar(signalStrengthUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Signal Strength characteristic: %s", err)
	}

	signalStrengthChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	signalStrengthChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving signal strength: %s", err)
			}
		}()

		log.Print("Got Signal Strength request.")

		sigStrength, err := commands.GetSignalStrength(unitId)
		if err != nil {
			return
		}

		log.Printf("Read Signal Strength: %s", sigStrength)

		resp = []byte(sigStrength)
		return
	})

	err = deviceService.AddChar(signalStrengthChar)
	if err != nil {
		log.Fatalf("Failed to add Signal Strength characteristic to device service: %s", err)
	}

	// Get wifi connection status
	wifiStatusChar, err := deviceService.NewChar(wifiStatusUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Wifi Connection Status characteristic: %s", err)
	}

	wifiStatusChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	wifiStatusChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving wifi connection status: %s", err)
			}
		}()

		log.Print("Got Wifi Connection Status request.")

		wifiConnectionState, err := commands.GetWifiStatus(unitId)
		if err != nil {
			return
		}

		log.Printf("Read Wifi Status: %s", wifiConnectionState)

		res := ""
		if wifiConnectionState.WPAState == "COMPLETED" {
			res = wifiConnectionState.SSID
		}

		resp = []byte(res)
		return
	})

	err = deviceService.AddChar(wifiStatusChar)
	if err != nil {
		log.Fatalf("Failed to add Get Wifi Status characteristic to device service: %s", err)
	}

	// set wifi connection
	setWifiChar, err := deviceService.NewChar(setWifiUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create set wifi characteristic: %s", err)
	}

	setWifiChar.Properties.Flags = []string{
		gatt.FlagCharacteristicEncryptAuthenticatedWrite,
	}

	setWifiChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error setting wifi connection: %s.", err)
			}
		}()

		var req commands.SetWifiRequest
		err = json.Unmarshal(value, &req)
		if err != nil {
			log.Printf("Error unmarshaling wi-fi payload: %s", err)
			return
		}

		if req.Network == "" || req.Password == "" {
			log.Printf("Missing network or password in wi-fi pairing request.")
			err = errors.New("missing network or password")
			return
		}

		newWifiList := []commands.WifiEntity{
			{
				Priority: 1,
				SSID:     req.Network,
				Psk:      req.Password,
			},
		}

		setWifiResp, err := commands.SetWifiConnection(unitId, newWifiList)
		if err != nil {
			log.Printf("Failed to set wifi connection: %s", err)
			return
		}

		if setWifiResp.Result {
			log.Printf("Wifi Connection set successfully: %s", req.Network)
		} else {
			log.Printf("Failed to set wifi connection: %s", err)
			return
		}

		resp = []byte(req.Network)
		return
	})

	err = deviceService.AddChar(setWifiChar)
	if err != nil {
		log.Fatalf("Failed to add Set Wifi characteristic to device service: %s", err)
	}

	// Vehicle service
	vehicleService, err := app.NewService(vehicleServiceUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create vehicle service: %s", err)
	}

	err = app.AddService(vehicleService)
	if err != nil {
		log.Fatalf("Failed to add vehicle service to app: %s", err)
	}

	// Get VIN
	vinChar, err := vehicleService.NewChar(vinCharUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create VIN characteristic: %s", err)
	}

	vinChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead}

	vinChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving VIN: %s", err)
			}
		}()

		if lastVIN != "" {
			resp = []byte(lastVIN)
			log.Printf("Returning cached VIN: %s", lastVIN)
			return
		}

		vin, protocol, err := commands.GetVIN(unitId)
		if err != nil {
			err = nil
			log.Printf("Unable to get VIN")
			resp = []byte("00000000000000000")
			return
		}

		log.Printf("Got Protocol: %s", protocol) // need to do something with protocol to set right template
		log.Printf("Got VIN: %s", vin)
		lastVIN = vin
		resp = []byte(vin)
		return
	})

	err = vehicleService.AddChar(vinChar)
	if err != nil {
		log.Fatalf("Failed to add VIN characteristic to vehicle service: %s", err)
	}

	// Diagnostic codes
	dtcChar, err := vehicleService.NewChar(diagCodeCharUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create diagnostic Code characteristic: %s", err)
	}

	dtcChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedRead, gatt.FlagCharacteristicEncryptAuthenticatedWrite}

	dtcChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving diagnostic codes: %s", err)
			}
		}()

		log.Print("Got diagnostic request")

		codes, err := commands.GetDiagnosticCodes(unitId)
		if err != nil {
			resp = []byte("0")
			return
		}

		log.Printf("Got Error Codes: %s", codes)

		resp = []byte(codes)
		return
	})

	dtcChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error clearing diagnostic codes hash: %s.", err)
			}
		}()

		log.Printf("Got clear DTC request")

		err = commands.ClearDiagnosticCodes(unitId)
		if err != nil {
			return
		}

		log.Printf("Cleared DTCs")

		return
	})

	err = vehicleService.AddChar(dtcChar)
	if err != nil {
		log.Fatalf("Failed to add diagnostic characteristic to vehicle service: %s", err)
	}

	// Sleep Control
	sleepControlChar, err := deviceService.NewChar(sleepControlUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create sleep control characteristic: %s", err)
	}

	sleepControlChar.Properties.Flags = []string{gatt.FlagCharacteristicEncryptAuthenticatedWrite}

	sleepControlChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error extending sleep time: %s.", err)
			}
		}()

		log.Printf("Got extend sleep request")

		err = commands.ExtendSleepTimer(unitId)
		if err != nil {
			return
		}

		log.Printf("Extended sleep time to 900 seconds")

		return
	})

	err = deviceService.AddChar(sleepControlChar)
	if err != nil {
		log.Fatalf("Failed to add sleep control characteristic to device service: %s", err)
	}

	// Transactions service
	transactionsService, err := app.NewService(transactionsServiceUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create transaction service: %s", err)
	}

	err = app.AddService(transactionsService)
	if err != nil {
		log.Fatalf("Failed to add transaction service to app: %s", err)
	}

	// Get Ethereum address
	addrChar, err := transactionsService.NewChar(addrCharUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create get ethereum address characteristic: %s", err)
	}

	addrChar.Properties.Flags = []string{
		gatt.FlagCharacteristicEncryptAuthenticatedRead,
	}

	addrChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		log.Print("Got address request")

		addr, err := commands.GetEthereumAddress(unitId)
		if err != nil {
			return
		}

		resp = addr[:]

		return
	})

	err = transactionsService.AddChar(addrChar)
	if err != nil {
		log.Fatalf("Failed to add Ethereum address characteristic: %s", err)
	}

	// Sign hash
	signChar, err := transactionsService.NewChar(signCharUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create sign hash characteristic: %s", err)
	}

	signChar.Properties.Flags = []string{
		gatt.FlagCharacteristicEncryptAuthenticatedWrite,
		gatt.FlagCharacteristicEncryptAuthenticatedRead,
	}

	signChar.OnWrite(func(c *service.Char, value []byte) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error signing hash: %s.", err)
			}
		}()

		// Wipe any old value so that if this fails, the client doesn't mistakenly
		// think everything is fine.
		lastSignature = nil

		if l := len(value); l != 32 {
			err = fmt.Errorf("input has byte length %d, must be 32", l)
			return
		}

		log.Printf("Got sign request for hash: %s.", hex.EncodeToString(value))

		sig, err := commands.SignHash(unitId, value)
		if err != nil {
			return
		}

		lastSignature = sig

		log.Printf("Signature: %s.", hex.EncodeToString(sig))

		return
	})

	signChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		log.Printf("Got read request for hash: %s.", hex.EncodeToString(lastSignature))
		resp = lastSignature
		return
	})

	err = transactionsService.AddChar(signChar)
	if err != nil {
		log.Fatalf("Failed to add hash signing characteristic: %s", err)
	}

	err = app.Run()
	if err != nil {
		log.Fatalf("Failed to initialize app: %s", err)
	}

	cancel, err := app.Advertise(math.MaxUint32)
	if err != nil {
		log.Fatalf("Failed advertising: %s", err)
	}

	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	//Check if we should disable new connections
	devices, err := app.Adapter().GetDevices()
	if err != nil {
		log.Fatalf("Could not retrieve previously paired devices: %s", err)
	}

	if coldBoot == false && hasPairedDevices(devices) {
		log.Printf("Disabling bonding")
		err = btManager.SetBondable(false)
		if err != nil {
			log.Fatalf("Failed to set bonding status: %s", err)
		}
	}

	adapterName, err := app.Adapter().GetName()
	if err != nil {
		log.Fatalf("Failed to get adapter name: %s", err)
	}

	adapterAlias, err := app.Adapter().GetAlias()
	if err != nil {
		log.Fatalf("Failed to get adapter alias: %s", err)
	}

	canBusInformation, err := commands.DetectCanbus(unitId)
	if err != nil {
		log.Printf("Failed to autodetect a canbus: %s", err)
	}

	err = btManager.SetAdvertising(true)
	if err != nil {
		log.Fatalf("failed to set advertising on the controller: %s", err)
	}

	log.Printf("Canbus Protocol Info: %v", canBusInformation)
	log.Printf("Adapter address: %s", app.Adapter().Properties.Address)
	log.Printf("Adapter name: %s, alias: %s", adapterName, adapterAlias)

	log.Printf("Device service: %s", deviceService.Properties.UUID)
	log.Printf("  Get Serial Number characteristic: %s", unitSerialChar.Properties.UUID)
	log.Printf("  Get Secondary ID characteristic: %s", secondSerialChar.Properties.UUID)
	log.Printf("  Get Hardware Revision characteristic: %s", hwRevisionChar.Properties.UUID)
	log.Printf("  Get Software Version characteristic: %s", softwareVersionChar.Properties.UUID)
	log.Printf("  Set Bluetooth Version characteristic: %s", bluetoothVersionChar.Properties.UUID)
	log.Printf("  Sleep Control characteristic: %s", sleepControlChar.Properties.UUID)

	log.Printf("  Get Signal Strength characteristic: %s", signalStrengthChar.Properties.UUID)
	log.Printf("  Get Wifi Connection Status characteristic: %s", wifiStatusChar.Properties.UUID)
	log.Printf("  Set Wifi Connection characteristic: %s", setWifiChar.Properties.UUID)

	log.Printf("Vehicle service: %s", vehicleService.Properties.UUID)
	log.Printf("  Get VIN characteristic: %s", vinChar.Properties.UUID)
	log.Printf("  Get DTC characteristic: %s", dtcChar.Properties.UUID)
	log.Printf("  Clear DTC characteristic: %s", dtcChar.Properties.UUID)

	log.Printf("Transactions service: %s", transactionsService.Properties.UUID)
	log.Printf("  Get ethereum address characteristic: %s", addrChar.Properties.UUID)
	log.Printf("  Sign hash characteristic: %s", signChar.Properties.UUID)

	sig := <-sigChan
	log.Printf("Terminating from signal: %s", sig)
}

// Utility Function
func hasPairedDevices(devices []*device.Device1) bool {
	for _, device := range devices {
		log.Printf("Found previously connected device: %v", device.Properties.Alias)
		if device.Properties.Trusted && device.Properties.Paired {
			return true
		}
	}
	return false
}

// Utility Function
func isColdBoot(unitID uuid.UUID) (result bool, err error) {
	status, httpError := commands.GetPowerStatus(unitID)
	for httpError != nil {
		status, httpError = commands.GetPowerStatus(unitID)
		time.Sleep(1 * time.Second)
	}

	log.Printf("Last Start Reason: %s", status.Spm.LastTrigger.Up)
	if status.Spm.LastTrigger.Up == "plug" {

		result = true
		return
	}
	result = false
	return
}
