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

	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/api/service"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"

	"github.com/sirupsen/logrus"
)

const (
	adapterID                  = "hci0"
	serviceName                = "edge-network"
	contentTypeJSON            = "application/json"
	getVINCommand              = `obd.query vin mode=09 pid=02 header=7DF bytes=20 formula='messages[0].data[3:].decode("ascii")' baudrate=500000 protocol=6 verify=false force=true`
	getEthereumAddressCommand  = `crypto.query ethereum_address`
	signHashCommand            = `crypto.sign_string `
	getSecondaryIdCommand      = `config.get device.id`
	getHardwareRevisionCommand = `config.get hw.version`
	autoPiBaseURL              = "http://192.168.4.1:9000"

	appUUIDSuffix = "-6859-4d6c-a87b-8d2c98c9f6f0"
	appUUIDPrefix = "5c30"

	deviceServiceUUIDFragment       = "7fa4"
	vehicleServiceUUIDFragment      = "d387"
	primaryIdCharUUIDFragment       = "5a11"
	secondaryIdCharUUIDFragment     = "5a12"
	hwVersionUUIDFragment           = "5a13"
	vinCharUUIDFragment             = "0acc"
	transactionsServiceUUIDFragment = "aade"
	addrCharUUIDFragment            = "1dd2"
	signCharUUIDFragment            = "e60f"
)

var lastSignature []byte

type unitIDResponse struct {
	UnitID string `json:"unit_id"`
}

type executeRawRequest struct {
	Command string `json:"command"`
}

type ethereumAddress [20]byte

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
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	req := executeRawRequest{Command: getHardwareRevisionCommand}

	var version float64

	err = executeRequest("POST", path, req, &version)
	if err != nil {
		return
	}

	hwRevision = fmt.Sprint(version)
	return
}

func getSecondarySerialNumber(unitID uuid.UUID) (id uuid.UUID, err error) {
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	req := executeRawRequest{Command: getSecondaryIdCommand}

	var strID string

	err = executeRequest("POST", path, req, &strID)
	if err != nil {
		return
	}

	id, err = uuid.Parse(strID)
	return
}

func getUnitID() (unitID uuid.UUID, err error) {
	resp := unitIDResponse{}

	err = executeRequest("GET", autoPiBaseURL, nil, &resp)
	if err != nil {
		return
	}

	unitID, err = uuid.Parse(resp.UnitID)
	return
}

func getVIN(unitID uuid.UUID) (vin string, err error) {
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	req := executeRawRequest{Command: getVINCommand}

	resp := executeRawResponse{}

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	vin = resp.Value
	if len(vin) != 17 {
		err = fmt.Errorf("response contained a VIN with %s characters", vin)
	}

	return
}

type executeRawResponse struct {
	Value string `json:"value"`
}

func signHash(unitID uuid.UUID, hash []byte) (sig []byte, err error) {
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	hashHex := hex.EncodeToString(hash)

	req := executeRawRequest{Command: signHashCommand + hashHex}

	resp := executeRawResponse{}

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	value := resp.Value
	value = strings.TrimPrefix(value, "0x")

	sig, err = hex.DecodeString(value)
	return
}

func getEthereumAddress(unitID uuid.UUID) (addr ethereumAddress, err error) {
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	req := executeRawRequest{Command: getEthereumAddressCommand}

	resp := executeRawResponse{}

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	addrString := resp.Value
	addrString = strings.TrimPrefix(addrString, "0x")

	addrSlice, err := hex.DecodeString(addrString)
	if err != nil {
		return
	}

	if l := len(addrSlice); l != 20 {
		err = fmt.Errorf("address has %d bytes", l)
		return
	}

	addr = *(*ethereumAddress)(addrSlice)

	return
}

func main() {
	log.Printf("Starting DIMO Edge Network")
	log.Printf("Sleeping for 30 seconds to allow D-Bus and BlueZ to start up")
	time.Sleep(30 * time.Second)

	// Used by go-bluetooth.
	// TODO(elffjs): Turn this off?
	logrus.SetLevel(logrus.DebugLevel)

	err := setupBluez()
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

		log.Print("Got Unit Serial request.")

		unitID, err := getUnitID()
		if err != nil {
			return
		}

		log.Printf("Read UnitId: %s", unitID)

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

		log.Print("Got Unit Secondary Id request.")

		unitID, err := getUnitID()
		if err != nil {
			return
		}

		deviceId, err := getSecondarySerialNumber(unitID)
		if err != nil {
			return
		}

		log.Printf("Read Secondary: %s", deviceId)

		resp = []byte(deviceId.String())
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

		log.Print("Got Hardware Revison request.")

		unitID, err := getUnitID()
		if err != nil {
			return
		}

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

		log.Print("Got VIN request.")

		unitID, err := getUnitID()
		if err != nil {
			return
		}

		vin, err := getVIN(unitID)
		if err != nil {
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
		log.Print("Got address request.")

		unitID, err := getUnitID()
		if err != nil {
			return
		}

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

		unitID, err := getUnitID()
		if err != nil {
			return
		}

		sig, err := signHash(unitID, value)
		if err != nil {
			return
		}

		lastSignature = sig

		log.Printf("Signature: %s.", hex.EncodeToString(sig))

		return
	})

	signChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
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

	log.Printf("Vehicle service: %s", vehicleService.Properties.UUID)
	log.Printf("  Get VIN characteristic: %s", vinChar.Properties.UUID)

	log.Printf("Transactions service: %s", transactionsService.Properties.UUID)
	log.Printf("  Get ethereum address characteristic: %s", addrChar.Properties.UUID)
	log.Printf("  Sign hash characteristic: %s", signChar.Properties.UUID)

	sig := <-sigChan
	log.Printf("Terminating from signal: %s", sig)
}
