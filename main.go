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

	"github.com/google/uuid"
	"github.com/muka/go-bluetooth/api/service"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
)

const (
	adapterID            = "hci0"
	serviceName          = "edge-network"
	jsonContentType      = "application/json"
	uuidPrefix           = "463e"
	uuidSuffix           = "-f894-44aa-92a2-0d7338075d74"
	serviceUUIDFragment  = "3f16"
	getVINUUIDFragment   = "de95"
	signHashUUIDFragment = "6fe3"
	getVINCommand        = `obd.query vin mode=09 pid=02 header=7DF bytes=20 formula='messages[0].data[3:].decode("ascii")' baudrate=500000 protocol=6 verify=false force=true`
	signHashCommand      = `crypto.sign_string `
	autoPiBaseURL        = "http://192.168.4.1:9000"
)

// The go-bluetooth library wants to construct ids for services
// and characteristics as follows:
//
// app prefix (2 bytes) + fragment (2 bytes) + app suffix (14 bytes)

// We need
// * Get VIN
// * Sign message

var lastSignature []byte

type unitIDResponse struct {
	UnitID string `json:"unit_id"`
}

type executeRawRequest struct {
	Command string `json:"command"`
}

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

	resp, err := http.Post(fmt.Sprintf("%s/dongle/%s/execute_raw", autoPiBaseURL, unitID), "application/json", reqBody)
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

	resp, err := http.Post(url, jsonContentType, reqBody)
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

func main() {
	opt := service.AppOptions{
		AdapterID:  adapterID,
		AgentCaps:  agent.CapNoInputNoOutput,
		UUIDSuffix: uuidSuffix,
		UUID:       uuidPrefix,
	}

	app, err := service.NewApp(opt)
	if err != nil {
		log.Fatal(err)
	}

	defer app.Close()

	app.SetName(serviceName)

	log.Printf("Bluetooth address: %s.", app.Adapter().Properties.Address)

	if !app.Adapter().Properties.Powered {
		log.Print("Bluetooth not powered, attempting to change.")
		err = app.Adapter().SetPowered(true)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Unclear whether we should split this up.
	svc, err := app.NewService(serviceUUIDFragment)
	if err != nil {
		log.Fatal(err)
	}

	err = app.AddService(svc)
	if err != nil {
		log.Fatal(err)
	}

	vinChar, err := svc.NewChar(getVINUUIDFragment)
	if err != nil {
		log.Fatal(err)
	}

	vinChar.Properties.Flags = []string{gatt.FlagCharacteristicRead}

	vinChar.OnRead(func(c *service.Char, options map[string]interface{}) (resp []byte, err error) {
		defer func() {
			if err != nil {
				log.Printf("Error retrieving VIN: %s.", err)
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

		log.Printf("Got VIN: %s.", vin)

		resp = []byte(vin)
		return
	})

	err = svc.AddChar(vinChar)
	if err != nil {
		log.Fatal(err)
	}

	signChar, err := svc.NewChar(signHashUUIDFragment)
	if err != nil {
		log.Fatal(err)
	}

	signChar.Properties.Flags = []string{
		gatt.FlagCharacteristicWrite,
		gatt.FlagCharacteristicRead,
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
		if len(lastSignature) == 0 {
			err = errors.New("no stored signature")
			return
		}

		resp = lastSignature
		return
	})

	err = svc.AddChar(signChar)
	if err != nil {
		log.Fatal(err)
	}

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}

	// First argument is a cancel function.
	cancel, err := app.Advertise(math.MaxUint32)
	if err != nil {
		log.Fatal(err)
	}

	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	log.Printf("Exposed service: %s.", svc.UUID)
	log.Printf("Get VIN characteristic: %s.", vinChar.UUID)
	log.Printf("Sign hash characteristic: %s.", signChar.UUID)

	sig := <-sigChan
	log.Printf("Got signal %s, terminating.", sig)
}
