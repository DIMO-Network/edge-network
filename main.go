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

const adapterID = "hci0"

const contentTypeJSON = "application/json"

// The go-bluetooth library wants to construct uuids for services
// and characteristics as follows:
//
// app prefix (2 bytes) + fragment (2 bytes) + app suffix (14 bytes)
const (
	uuidPrefix           = "463e"
	uuidSuffix           = "-f894-44aa-92a2-0d7338075d74"
	serviceUUIDFragment  = "3f16"
	getVINUUIDFragment   = "de95"
	signHashUUIDFragment = "6fe3"
)

const (
	autoPiBaseURL   = "http://192.168.4.1:9000"
	getVINCommand   = `obd.query vin mode=09 pid=02 header=7DF bytes=20 formula='messages[0].data[3:].decode("ascii")' baudrate=500000 protocol=6 verify=false force=true`
	signHashCommand = `crypto.sign_string `
)

// lastSignature stores the last signature computed by the process.
// It may be empty and is not thread-safe at all.
var lastSignature []byte

type unitIDResponse struct {
	UnitID string `json:"unit_id"`
}

type executeRawRequest struct {
	Command string `json:"command"`
}

type executeRawResponse struct {
	Value string `json:"value"`
}

func getUnitID() (unitID uuid.UUID, err error) {
	respObj := unitIDResponse{}

	err = request("GET", autoPiBaseURL, nil, &respObj)
	if err != nil {
		return
	}

	unitID, err = uuid.Parse(respObj.UnitID)
	return
}

func getVIN(unitID uuid.UUID) (vin string, err error) {
	reqObj := executeRawRequest{Command: getVINCommand}
	respObj := executeRawResponse{}

	err = request("POST", fmt.Sprintf("%s/dongle/%s/execute_raw", autoPiBaseURL, unitID), reqObj, &respObj)
	if err != nil {
		return
	}

	vin = respObj.Value
	if len(vin) != 17 {
		err = fmt.Errorf("response contained a VIN with %s characters", vin)
	}

	return
}

func signHash(unitID uuid.UUID, hash []byte) (sig []byte, err error) {
	if l := len(hash); l != 32 {
		err = fmt.Errorf("hash has length %d", l)
		return
	}
	hashHex := hex.EncodeToString(hash)

	reqObj := executeRawRequest{Command: signHashCommand + hashHex}
	respObj := executeRawResponse{}

	err = request("POST", fmt.Sprintf("%s/dongle/%s/execute_raw", autoPiBaseURL, unitID), reqObj, &respObj)
	if err != nil {
		return
	}

	value := respObj.Value
	value = strings.TrimPrefix(value, "0x")

	sig, err = hex.DecodeString(value)
	return
}

func request(method, url string, reqObj, respObj any) (err error) {
	var reqBody io.Reader

	if reqObj != nil {
		buf := new(bytes.Buffer)
		err = json.NewEncoder(buf).Encode(reqObj)
		if err != nil {
			return
		}
		reqBody = buf
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return
	}

	if reqObj != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if c := resp.StatusCode; c != 200 {
		return fmt.Errorf("status code %d", c)
	}

	err = json.NewDecoder(resp.Body).Decode(respObj)
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
		defer func() {
			if err != nil {
				log.Printf("Error retrieving signature: %s.", err)
			}
		}()

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
