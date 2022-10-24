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

	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/api/service"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"

	"github.com/sirupsen/logrus"

	"github.com/muka/go-bluetooth/hw"
)

const (
	adapterID                 = "hci0"
	serviceName               = "edge-network"
	contentTypeJSON           = "application/json"
	getVINCommand             = `obd.query vin mode=09 pid=02 header=7DF bytes=20 formula='messages[0].data[3:].decode("ascii")' baudrate=500000 protocol=6 verify=false force=true`
	getEthereumAddressCommand = `crypto.query ethereum_address`
	signHashCommand           = `crypto.sign_string `
	autoPiBaseURL             = "http://192.168.4.1:9000"
)

var lastSignature []byte

type unitIDResponse struct {
	UnitID string `json:"unit_id"`
}

type executeRawRequest struct {
	Command string `json:"command"`
}

type ethereumAddress [20]byte

func getUnitID() (unitID uuid.UUID, err error) {
	resp, err := http.Get(autoPiBaseURL)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("status code %d", resp.StatusCode)
		return
	}

	respObj := unitIDResponse{}

	err = json.NewDecoder(resp.Body).Decode(&respObj)
	if err != nil {
		return
	}

	unitID, err = uuid.Parse(respObj.UnitID)

	if err == nil {
		log.Printf("Got unit id: %s", unitID)
	}

	return
}

func getVIN(unitID uuid.UUID) (vin string, err error) {
	reqBody := new(bytes.Buffer)
	err = json.NewEncoder(reqBody).Encode(executeRawRequest{Command: getVINCommand})
	if err != nil {
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/dongle/%s/execute_raw", autoPiBaseURL, unitID), contentTypeJSON, reqBody)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bs, _ := io.ReadAll(resp.Body)
		fmt.Print(string(bs))

		err = fmt.Errorf("status code %d", resp.StatusCode)
		return
	}

	respObj := executeRawResponse{}

	err = json.NewDecoder(resp.Body).Decode(&respObj)
	if err != nil {
		return
	}

	vin = respObj.Value
	if len(vin) != 17 {
		err = fmt.Errorf("response contained a VIN with %s characters", vin)
	}

	return
}

type executeRawResponse struct {
	Value string `json:"value"`
}

func signHash(unitID uuid.UUID, hash []byte) (sig []byte, err error) {
	hashHex := hex.EncodeToString(hash)

	url := fmt.Sprintf("%s/dongle/%s/execute_raw", autoPiBaseURL, unitID)

	reqBody := new(bytes.Buffer)
	err = json.NewEncoder(reqBody).Encode(executeRawRequest{Command: signHashCommand + hashHex})
	if err != nil {
		return
	}

	resp, err := http.Post(url, contentTypeJSON, reqBody)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bs, _ := io.ReadAll(resp.Body)
		fmt.Print(string(bs))

		err = fmt.Errorf("status code %d", resp.StatusCode)
		return
	}

	respObj := executeRawResponse{}

	err = json.NewDecoder(resp.Body).Decode(&respObj)
	if err != nil {
		return
	}

	value := respObj.Value
	value = strings.TrimPrefix(value, "0x")

	sig, err = hex.DecodeString(value)
	return
}

func getEthereumAddress(unitID uuid.UUID) (addr ethereumAddress, err error) {
	reqBody := new(bytes.Buffer)
	err = json.NewEncoder(reqBody).Encode(executeRawRequest{Command: getEthereumAddressCommand})
	if err != nil {
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/dongle/%s/execute_raw", autoPiBaseURL, unitID), contentTypeJSON, reqBody)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bs, _ := io.ReadAll(resp.Body)
		fmt.Print(string(bs))

		err = fmt.Errorf("status code %d", resp.StatusCode)
		return
	}

	respObj := executeRawResponse{}

	err = json.NewDecoder(resp.Body).Decode(&respObj)
	if err != nil {
		return
	}

	log.Printf("Got from crypto.query ethereum_address: %s", respObj.Value)

	addrString := respObj.Value
	strings.TrimPrefix(addrString, "0x") // Might not be necessary.

	addrSlice, err := hex.DecodeString(addrString)
	if l := len(addrSlice); l != 20 {
		err = fmt.Errorf("address has %d bytes", l)
		return
	}

	addr = *(*ethereumAddress)(addrSlice)

	return
}

func main() {
	// Used by go-bluetooth.
	logrus.SetLevel(logrus.DebugLevel)

	btmgmt := hw.NewBtMgmt(adapterID)
	if len(os.Getenv("DOCKER")) > 0 {
		btmgmt.BinPath = "./bin/docker-btmgmt"
	}

	// set LE mode
	btmgmt.SetPowered(false)
	btmgmt.SetLe(true)
	btmgmt.SetBredr(false)
	btmgmt.SetPowered(true)
	btmgmt.SetConnectable(true)
	btmgmt.SetBondable(true)
	btmgmt.SetPairable(true)
	btmgmt.SetSc(true)

	app, err := service.NewApp(adapterID)
	if err != nil {
		log.Fatal(err)
	}

	defer app.Close()

	app.AgentCaps = agent.CapDisplayYesNo
	app.AgentSetAsDefault = true // Already set in NewApp, but just to be explicit..

	log.Printf("Adapter address: %s", app.Adapter().Properties.Address)

	if !app.Adapter().Properties.Powered {
		log.Print("Adapter not powered.")
		err = app.Adapter().SetPowered(true)
		if err != nil {
			log.Fatalf("Failed to power adapter: %s", err)
		}
	}

	// Presently unused, but will hold things like cell and wifi status and settings.
	deviceService, err := app.NewService()
	if err != nil {
		log.Fatalf("Failed to create device service: %s", err)
	}

	err = app.AddService(deviceService)
	if err != nil {
		log.Fatalf("Failed to add device service to app: %s", err)
	}

	vehicleService, err := app.NewService()
	if err != nil {
		log.Fatalf("Failed to create vehicle service: %s", err)
	}

	err = app.AddService(vehicleService)
	if err != nil {
		log.Fatalf("Failed to add vehicle service to app: %s", err)
	}

	vinChar, err := vehicleService.NewChar()
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

	transactionsService, err := app.NewService()
	if err != nil {
		log.Fatalf("Failed to create transaction service: %s", err)
	}

	addrChar, err := transactionsService.NewChar()
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

	signChar, err := transactionsService.NewChar()
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
		log.Fatal(err)
	}

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}

	cancel, err := app.Advertise(math.MaxUint32)
	if err != nil {
		log.Fatal(err)
	}

	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	log.Printf("Device service: %s", deviceService.Properties.UUID)

	log.Printf("Vehicle service: %s", vehicleService.Properties.UUID)
	log.Printf("  Get VIN characteristic: %s", vinChar.Properties.UUID)

	log.Printf("Transactions service: %s", transactionsService.Properties.UUID)
	log.Printf("  Get ethereum address characteristic: %s", addrChar.Properties.UUID)
	log.Printf("  Sign hash characteristic: %s", signChar.Properties.UUID)

	sig := <-sigChan
	log.Printf("Terminating from signal: %s", sig)
}
