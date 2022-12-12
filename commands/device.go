package commands

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/google/uuid"
)

func GetDeviceName() (bluetoothName string, unitId uuid.UUID) {
	unitIDBytes, err := os.ReadFile("/etc/salt/minion_id")
	if err != nil {
		log.Fatalf("Could not read unit ID from file: %s", err)
	}

	unitIDBytes = bytes.TrimSpace(unitIDBytes)

	unitID, err := uuid.ParseBytes(unitIDBytes)
	if err != nil {
		log.Fatalf("Invalid unit id: %s", err)
	}

	unitIDStr := unitID.String()
	return "autopi-" + unitIDStr[len(unitIDStr)-12:], unitID
}

func GetHardwareRevision(unitID uuid.UUID) (hwRevision string, err error) {
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

func GetSoftwareVersion(unitID uuid.UUID) (version string, err error) {
	req := executeRawRequest{Command: getSoftwareVersionCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp string

	err = executeRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	version = resp

	return
}

func GetDeviceID(unitID uuid.UUID) (deviceID uuid.UUID, err error) {
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

func ExtendSleepTimer(unitID uuid.UUID) (err error) {
	req := executeRawRequest{Command: sleepTimerDelayCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func AnnounceCode(unitID uuid.UUID, code uint32) (err error) {
	announcement := `audio.speak 'Pin Code , `

	stringCode := strconv.Itoa(int(code))

	for _, digit := range stringCode {
		announcement += string(digit) + ` , `
	}

	announcement += `'`
	log.Printf("Announcement Command: %s", announcement)
	req := executeRawRequest{Command: announcement}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func GetSignalStrength(unitID uuid.UUID) (sigStrength string, err error) {
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
func GetWifiStatus(unitID uuid.UUID) (connectionObject wifiConnectionsResponse, err error) {
	req := executeRawRequest{Command: wifiStatusCommand, Arg: nil}
	path := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)

	var resp wifiConnectionsResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	connectionObject = resp
	return
}

func SetWifiConnection(unitID uuid.UUID, newConnectionList []WifiEntity) (connectionObject setWifiConnectionResponse, err error) {
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

func GetPowerStatus(unitID uuid.UUID) (responseObject powerStatusResponse, err error) {
	req := executeRawRequest{Command: powerStatusCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)

	var resp powerStatusResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	responseObject = resp
	return
}
