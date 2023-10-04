package commands

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/google/uuid"
)

func GetDeviceName(logger zerolog.Logger) (bluetoothName string, unitID uuid.UUID) {
	unitIDBytes, err := os.ReadFile("/etc/salt/minion_id")
	if err != nil {
		logger.Fatal().Err(err).Msgf("Could not read unit ID from file: %s", err)
	}

	unitIDBytes = bytes.TrimSpace(unitIDBytes)

	unitID, err = uuid.ParseBytes(unitIDBytes)
	if err != nil {
		logger.Fatal().Err(err).Msgf("Invalid unit id: %s", err)
	}

	unitIDStr := unitID.String()
	return "autopi-" + unitIDStr[len(unitIDStr)-12:], unitID
}

func GetHardwareRevision(unitID uuid.UUID) (hwRevision string, err error) {
	req := api.ExecuteRawRequest{Command: api.GetHardwareRevisionCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp float64

	err = api.ExecuteRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	hwRevision = fmt.Sprint(resp)
	return
}

func GetSoftwareVersion(unitID uuid.UUID) (version string, err error) {
	req := api.ExecuteRawRequest{Command: api.GetSoftwareVersionCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp string

	err = api.ExecuteRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	version = resp

	return
}

func GetDeviceID(unitID uuid.UUID) (deviceID uuid.UUID, err error) {
	req := api.ExecuteRawRequest{Command: api.GetDeviceIDCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp string

	err = api.ExecuteRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	deviceID, err = uuid.Parse(resp)
	return
}

func ExtendSleepTimer(unitID uuid.UUID) (err error) {
	req := api.ExecuteRawRequest{Command: api.SleepTimerDelayCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ExecuteRawResponse

	err = api.ExecuteRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func AnnounceCode(unitID uuid.UUID, intro string, code uint32, logger zerolog.Logger) (err error) {
	announcement := `audio.speak '` + intro + ` , `

	stringCode := strconv.Itoa(int(code))

	for _, digit := range stringCode {
		announcement += string(digit) + ` , `
	}

	announcement += `'`
	logger.Info().Msgf("Announcement Command: %s", announcement)
	req := api.ExecuteRawRequest{Command: announcement}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ExecuteRawResponse

	err = api.ExecuteRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func GetSignalStrength(unitID uuid.UUID) (sigStrength string, err error) {
	req := api.ExecuteRawRequest{Command: api.SignalStrengthCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.SignalStrengthResponse
	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	sigStrength = fmt.Sprint(resp.Current.Value)
	return
}

// Wifi
func GetWifiStatus(unitID uuid.UUID) (connectionObject api.WifiConnectionsResponse, err error) {
	req := api.ExecuteRawRequest{Command: api.WifiStatusCommand, Arg: nil}
	path := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)

	var resp api.WifiConnectionsResponse
	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	connectionObject = resp
	return
}

func SetWifiConnection(unitID uuid.UUID, newConnectionList []api.WifiEntity) (connectionObject api.SetWifiConnectionResponse, err error) {
	arg := []interface{}{
		"wpa_supplicant:networks",
		newConnectionList,
	}
	path := fmt.Sprintf("/dongle/%s/execute/", unitID)

	req := api.ExecuteRawRequest{Command: api.SetWifiConnectionCommand, Arg: arg, Kwarg: api.KwargType{
		Destructive: true,
		Force:       true,
	}}

	err = api.ExecuteRequest("POST", path, req, &connectionObject)
	if err != nil {
		return
	}

	return
}

func GetPowerStatus(unitID uuid.UUID) (responseObject api.PowerStatusResponse, err error) {
	req := api.ExecuteRawRequest{Command: api.PowerStatusCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)

	var resp api.PowerStatusResponse
	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	// check both stn and spm for voltage, return the one that has it, new property for voltagefound
	if resp.Stn.Battery.Voltage > 0 {
		resp.VoltageFound = resp.Stn.Battery.Voltage
	} else {
		resp.VoltageFound = resp.Spm.Battery.Voltage
	}
	responseObject = resp
	return
}

func GetIMSI(unitID uuid.UUID) (imsi string, err error) {
	req := api.ExecuteRawRequest{Command: api.GetModemCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var modem string
	err = api.ExecuteRequest("POST", url, req, &modem)
	if err != nil {
		return
	}

	var resp api.ExecuteRawResponse
	if modem == "ec2x" {
		req = api.ExecuteRawRequest{Command: api.Ec2xIMSICommand}
	} else {
		req = api.ExecuteRawRequest{Command: api.NormalIMSICommand}
	}

	err = api.ExecuteRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	imsi = resp.Data
	return
}
