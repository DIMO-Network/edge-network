package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	wifiStatusCommand          = `wifi.status`
	setWifiConnectionCommand   = `grains.set`
	getAvailableWifiCommand    = `grains.get`
	getDiagnosticCodeCommand   = `obd.dtc`
	clearDiagnosticCodeCommand = `obd.dtc clear=true`
	powerStatusCommand         = `power.status`
	autoPiBaseURL              = "http://192.168.4.1:9000"

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
	vinCharUUIDFragment             = "0acc"
	diagCodeCharUUIDFragment        = "0add"
	transactionsServiceUUIDFragment = "aade"
	addrCharUUIDFragment            = "1dd2"
	signCharUUIDFragment            = "e60f"
)

var lastSignature []byte
var lastVIN string

var unitID uuid.UUID

type KwargType struct {
	Destructive bool `json:"destructive,omitempty"`
	Force       bool `json:"force,omitempty"`
}
type executeRawRequest struct {
	Command string        `json:"command"`
	Arg     []interface{} `json:"arg"`
	Kwarg   KwargType     `json:"kwarg"`
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
	WPAState string `json:"wpa_state"`
	SSID     string `json:"ssid"`
}

type wifiEntity struct {
	Priority int    `json:"priority"`
	Psk      string `json:"psk"`
	SSID     string `json:"ssid"`
}

type setWifiConnectionResponse struct {
	Comment string `json:"comment"`
	Result  bool   `json:"result"`
	Changes struct {
		WPASupplicant struct {
			Networks []wifiEntity
		} `json:"wpa_supplicant"`
	}
}

type setWifiRequest struct {
	Network  string `json:"network"`
	Password string `json:"password"`
}

type dtcResponse struct {
	Stamp  string `json:"_stamp"`
	Type   string `json:"_type"`
	Values []struct {
		Code string `json:"code"`
		Text string `json:"text"`
	} `json:"values"`
}

type powerStatusResponse struct {
	Rpi struct {
		Uptime struct {
			Days     int    `json:"days"`
			Seconds  int    `json:"seconds"`
			SinceIso string `json:"since_iso"`
			SinceT   int    `json:"since_t"`
			Time     string `json:"time"`
			Users    int    `json:"users"`
		} `json:"uptime"`
	} `json:"rpi"`
	Spm struct {
		Battery struct {
			Level   int     `json:"level"`
			State   string  `json:"state"`
			Voltage float64 `json:"voltage"`
		} `json:"battery"`
		CurrentState string `json:"current_state"`
		LastState    struct {
			Down string `json:"down"`
			Up   string `json:"up"`
		} `json:"last_state"`
		LastTrigger struct {
			Down string `json:"down"`
			Up   string `json:"up"`
		} `json:"last_trigger"`
		SleepInterval int     `json:"sleep_interval"`
		Version       string  `json:"version"`
		VoltFactor    float64 `json:"volt_factor"`
		VoltTriggers  struct {
			HibernateLevel struct {
				Duration  int     `json:"duration"`
				Threshold float64 `json:"threshold"`
			} `json:"hibernate_level"`
			WakeChange struct {
				Difference float64 `json:"difference"`
				Period     int     `json:"period"`
			} `json:"wake_change"`
			WakeLevel struct {
				Duration  int     `json:"duration"`
				Threshold float64 `json:"threshold"`
			} `json:"wake_level"`
		} `json:"volt_triggers"`
	} `json:"spm"`
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
func getWifiStatus(unitID uuid.UUID) (connectionObject wifiConnectionsResponse, err error) {
	req := executeRawRequest{Command: wifiStatusCommand, Arg: nil}
	path := fmt.Sprintf("/dongle/%s/execute/", unitID)

	var resp wifiConnectionsResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	connectionObject = resp
	return
}

func setWifiConnection(unitID uuid.UUID, newConnectionList []wifiEntity) (connectionObject setWifiConnectionResponse, err error) {
	arg := []interface{}{
		"wpa_supplicant:networks",
		newConnectionList,
	}
	path := fmt.Sprintf("/dongle/%s/execute/", unitID)

	req := executeRawRequest{Command: setWifiConnectionCommand, Arg: arg, Kwarg: KwargType{
		Destructive: true,
		Force:       true,
	}}

	err = executeRequest("POST", path, req, &connectionObject)
	if err != nil {
		return
	}

	return
}

func clearDiagnosticCodes(unitID uuid.UUID) (err error) {
	req := executeRawRequest{Command: clearDiagnosticCodeCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func getDiagnosticCodes(unitID uuid.UUID) (codes string, err error) {
	req := executeRawRequest{Command: getDiagnosticCodeCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp dtcResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	log.Print("Response", resp)
	formattedResponse := ""
	for _, s := range resp.Values {
		formattedResponse += s.Code + ","
	}
	codes = strings.TrimSuffix(formattedResponse, ",")
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

func getPowerStatus(unitID uuid.UUID) (responseObject powerStatusResponse, err error) {
	req := executeRawRequest{Command: powerStatusCommand}
	path := fmt.Sprintf("/dongle/%s/execute/", unitID)

	var resp powerStatusResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	responseObject = resp
	return
}

// Utility Function
func isColdBoot(unitID uuid.UUID) (result bool, err error) {
	status, httpError := getPowerStatus(unitID)
	counter := 0
	for httpError != nil && counter < 30 {
		counter++
		status, httpError = getPowerStatus(unitID)
		log.Printf("Status: %v", status)
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

	return nil
}

func main() {
	log.Printf("Starting DIMO Edge Network")

	name := getDeviceName()

	bondable, err := isColdBoot(unitID)
	if err != nil {
		log.Fatalf("Failed to get power management status: %s", err)
	}
	log.Printf("Bluetooth name: %s, Bondable: %v", name, bondable)

	// Used by go-bluetooth.
	// TODO(elffjs): Turn this off?
	logrus.SetLevel(logrus.DebugLevel)

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

	// Get wifi connection status
	wifiStatusChar, err := deviceService.NewChar(wifiStatusUUIDFragment)
	if err != nil {
		log.Fatalf("Failed to create Wifi Connection Status characteristic: %s", err)
	}

	wifiStatusChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	wifiStatusChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving wifi connection status: %s", err)
			}
		}()

		log.Print("Got Wifi Connection Status request.")

		wifiConnectionState, err := getWifiStatus(unitID)
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

		var req setWifiRequest
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

		newWifiList := []wifiEntity{
			{
				Priority: 1,
				SSID:     req.Network,
				Psk:      req.Password,
			},
		}

		setWifiResp, err := setWifiConnection(unitID, newWifiList)
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

		codes, err := getDiagnosticCodes(unitID)
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

		err = clearDiagnosticCodes(unitID)
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

	if bondable == false {
		btmgmt := hw.NewBtMgmt(adapterID)
		err := btmgmt.SetBondable(false)
		if err != nil {
			log.Fatalf("Failed to set bonding status: %s", err)
		}
	}

	log.Printf("Device service: %s", deviceService.Properties.UUID)
	log.Printf("  Get Serial Number characteristic: %s", unitSerialChar.Properties.UUID)
	log.Printf("  Get Secondary ID characteristic: %s", secondSerialChar.Properties.UUID)
	log.Printf("  Get Hardware Revision characteristic: %s", hwRevisionChar.Properties.UUID)
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
