package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/api/service"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"

	"github.com/sirupsen/logrus"

	"github.com/muka/go-bluetooth/hw"
)

const (
	adapterID                  = "hci0"
	contentTypeJSON            = "application/json"
	getVINCommand              = `obd.query vin mode=09 pid=02 header=7DF bytes=20 formula='messages[0].data[3:].decode("ascii")' baudrate=500000 protocol=6 verify=false force=true`
	getEthereumAddressCommand  = `crypto.query ethereum_address`
	signHashCommand            = `crypto.sign_string `
	getDeviceIDCommand         = `config.get device.id`
	getHardwareRevisionCommand = `config.get hw.version`
	signalStrengthCommand      = `qmi.signal_strength`
	getConnectionListCommand   = `grains.get`
	wifiStatusCommand          = `wifi.status`
	autoPiBaseURL              = "http://192.168.4.1:9000"

	appUUIDSuffix = "-6859-4d6c-a87b-8d2c98c9f6f0"
	appUUIDPrefix = "5c30"

	deviceServiceUUIDFragment       = "7fa4"
	vehicleServiceUUIDFragment      = "d387"
	primaryIdCharUUIDFragment       = "5a11"
	secondaryIdCharUUIDFragment     = "5a12"
	hwVersionUUIDFragment           = "5a13"
	signalStrengthUUIDFragment      = "5a14"
	connectedWifiUUIDFragment       = "5a15"
	vinCharUUIDFragment             = "0acc"
	transactionsServiceUUIDFragment = "aade"
	addrCharUUIDFragment            = "1dd2"
	signCharUUIDFragment            = "e60f"
)

var lastSignature []byte
var lastVIN string

var unitID uuid.UUID

type executeRawRequest struct {
	Command string   `json:"command"`
	Arg     []string `json:"arg"`
	Kwarg   struct{} `json:"kwarg"`
}

// For some reason, this only gets returned for some calls.
type executeRawResponse struct {
	Value string `json:"value"`
}

type GenericSignalStrengthResponse struct {
	Network string  `json:"network"`
	Unit    string  `json:"unit"`
	Value   float64 `json:"value"`
}

type signalStrengthResponse struct {
	Current GenericSignalStrengthResponse
}

type wifiConnectionsResponse struct {
	WpaState string `json:"wpa_state"`
}

func executeRequest(method, path string, reqVal, respVal any) (err error) {
	var reqBody io.Reader

	if reqVal != nil {
		reqBuf := new(bytes.Buffer)
		err = json.NewEncoder(reqBuf).Encode(reqVal)
		if err != nil {
			return
		}
		reqBody = reqBuf
	}

	req, err := http.NewRequest(method, autoPiBaseURL+path, reqBody)
	if err != nil {
		return
	}

	if reqVal != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if c := resp.StatusCode; c >= 400 {
		return fmt.Errorf("status code %d", c)
	}

	if respVal == nil {
		return
	}

	err = json.NewDecoder(resp.Body).Decode(respVal)
	return
}

func getHardwareRevision(unitID uuid.UUID) (hwRevision string, err error) {
	req := executeRawRequest{Command: getHardwareRevisionCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp float64

	err = executeRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	hwRevision = fmt.Sprint(resp)
	return
}

func getDeviceID(unitID uuid.UUID) (deviceID uuid.UUID, err error) {
	req := executeRawRequest{Command: getDeviceIDCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp string

	err = executeRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	deviceID, err = uuid.Parse(resp)
	return
}

func getVIN(unitID uuid.UUID) (vin string, err error) {
	if lastVIN != "" {
		vin = lastVIN
		return
	}
	req := executeRawRequest{Command: getVINCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	vin = resp.Value
	lastVIN = resp.Value
	if len(vin) != 17 {
		err = fmt.Errorf("response contained a VIN with %s characters", vin)
	}

	return
}

func signHash(unitID uuid.UUID, hash []byte) (sig []byte, err error) {
	hashHex := hex.EncodeToString(hash)

	req := executeRawRequest{Command: signHashCommand + hashHex}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	sig = common.FromHex(resp.Value)
	return
}

func getEthereumAddress(unitID uuid.UUID) (addr common.Address, err error) {
	req := executeRawRequest{Command: getEthereumAddressCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	addr = common.HexToAddress(resp.Value)
	return
}

func getSignalStrength(unitID uuid.UUID) (sigStrength string, err error) {
	req := executeRawRequest{Command: signalStrengthCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp signalStrengthResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	sigStrength = fmt.Sprint(resp.Current.Value)
	return
}

// Wifi
func isWifiConnected(unitID uuid.UUID) (connectionState string, err error) {
	req := executeRawRequest{Command: wifiStatusCommand, Arg: make([]string, 0)}
	path := fmt.Sprintf("/dongle/%s/execute/", unitID)

	var resp wifiConnectionsResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	connectionState = fmt.Sprint(resp.WpaState)
	return
}

func getDeviceName() string {
	unitIDBytes, err := os.ReadFile("/etc/salt/minion_id")
	if err != nil {
		log.Fatalf("Could not read unit ID from file: %s", err)
	}

	unitIDBytes = bytes.TrimSpace(unitIDBytes)

	unitID, err = uuid.ParseBytes(unitIDBytes)
	if err != nil {
		log.Fatalf("Invalid unit id: %s", err)
	}

	unitIDStr := unitID.String()
	return "autopi-" + unitIDStr[len(unitIDStr)-12:]
}

func setupBluez(name string) error {
	btmgmt := hw.NewBtMgmt(adapterID)

	// Need to turn off the controller to be able to modify the next few settings.
	err := btmgmt.SetPowered(false)
	if err != nil {
		return fmt.Errorf("failed to power off the controller: %w", err)
	}

	err = btmgmt.SetLe(true)
	if err != nil {
		return fmt.Errorf("failed to enable LE: %w", err)
	}

	err = btmgmt.SetBredr(false)
	if err != nil {
		return fmt.Errorf("failed to disable BR/EDR: %w", err)
	}

	err = btmgmt.SetSc(true)
	if err != nil {
		return fmt.Errorf("failed to enable Secure Connections: %w", err)
	}

	err = btmgmt.SetName(name)
	if err != nil {
		return fmt.Errorf("failed to set name: %w", err)
	}

	err = btmgmt.SetPowered(true)
	if err != nil {
		return fmt.Errorf("failed to power on the controller: %w", err)
	}

	err = btmgmt.SetConnectable(true)
	if err != nil {
		return fmt.Errorf("failed to set the controller as connectable: %w", err)
	}

	err = btmgmt.SetBondable(true)
	if err != nil {
		return fmt.Errorf("failed to set the controller as bondable: %w", err)
	}

	err = btmgmt.SetDiscoverable(true)
	if err != nil {
		return fmt.Errorf("failed to set the controller as discoverable: %w", err)
	}

	err = btmgmt.SetAdvertising(true)
	if err != nil {
		return fmt.Errorf("failed to set the controller as advertising: %w", err)
	}

	return nil
}

func main() {
	log.Printf("Starting DIMO Edge Network")

	name := getDeviceName()

	log.Printf("Bluetooth name: %s", name)

	log.Printf("Sleeping for 10 seconds to allow D-Bus and BlueZ to start up")
	time.Sleep(10 * time.Second)

	// Used by go-bluetooth.
	// TODO(elffjs): Turn this off?
	logrus.SetLevel(logrus.DebugLevel)

	err := setupBluez(name)
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

	log.Printf("Adapter address: %s", app.Adapter().Properties.Address)

	if !app.Adapter().Properties.Powered {
		log.Print("Adapter not powered, attempting to power")
		err = app.Adapter().SetPowered(true)
		if err != nil {
			log.Fatalf("Failed to power adapter: %s", err)
		}
	}

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

		resp = []byte(unitID.String())
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

		deviceID, err := getDeviceID(unitID)
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

		hwRevision, err := getHardwareRevision(unitID)
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

		sigStrength, err := getSignalStrength(unitID)
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

	// Get connected wifi
	wifiConnectedChar, err := deviceService.NewChar(connectedWifiUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Wifi Connection Status characteristic: %s", err)
	}

	wifiConnectedChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	wifiConnectedChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving wifi connection status: %s", err)
			}
		}()

		log.Print("Got Wifi Connection Status request.")

		wifiConnectionState, err := isWifiConnected(unitID)
		if err != nil {
			return
		}

		log.Printf("Read Wifi Status: %s", wifiConnectionState)

		res := fmt.Sprintf("0x%x", 0)
		if wifiConnectionState != "DISCONNECTED" {
			res = fmt.Sprintf("0x%x", 1)
		}
		resp = []byte(res)

		return
	})

	err = deviceService.AddChar(wifiConnectedChar)
	if err != nil {
		log.Fatalf("Failed to add Get Wifi Status characteristic to device service: %s", err)
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

		log.Print("Got VIN request")

		fakeVin, err := os.ReadFile("/tmp/FAKE_VIN")
		stringFakeVin := strings.Trim(string(fakeVin), "")
		if err == nil && len(stringFakeVin) != 0 {
			log.Printf("Fake VIN: %s", stringFakeVin)
			resp = []byte(stringFakeVin)
			return
		}

		vin, err := getVIN(unitID)
		if err != nil {
			err = nil
			log.Printf("Unable to get VIN")
			resp = []byte("00000000000000000")
			return
		}

		log.Printf("Got VIN: %s", vin)

		resp = []byte(vin)
		return
	})

	err = vehicleService.AddChar(vinChar)
	if err != nil {
		log.Fatalf("Failed to add VIN characteristic to vehicle service: %s", err)
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

		addr, err := getEthereumAddress(unitID)
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

		sig, err := signHash(unitID, value)
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

	log.Printf("Device service: %s", deviceService.Properties.UUID)
	log.Printf("  Get Serial Number characteristic: %s", unitSerialChar.Properties.UUID)
	log.Printf("  Get Secondary ID characteristic: %s", secondSerialChar.Properties.UUID)
	log.Printf("  Get Hardware Revision characteristic: %s", hwRevisionChar.Properties.UUID)
	log.Printf("  Get Signal Strength characteristic: %s", signalStrengthChar.Properties.UUID)
	log.Printf("  Get Wifi Connection Status characteristic: %s", wifiConnectedChar.Properties.UUID)

	log.Printf("Vehicle service: %s", vehicleService.Properties.UUID)
	log.Printf("  Get VIN characteristic: %s", vinChar.Properties.UUID)

	log.Printf("Transactions service: %s", transactionsService.Properties.UUID)
	log.Printf("  Get ethereum address characteristic: %s", addrChar.Properties.UUID)
	log.Printf("  Sign hash characteristic: %s", signChar.Properties.UUID)

	sig := <-sigChan
	log.Printf("Terminating from signal: %s", sig)
}
