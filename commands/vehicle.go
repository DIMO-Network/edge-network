package commands

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

func DetectCanbus(unitID uuid.UUID) (canbusInfo CanbusInfo, err error) {
	req := executeRawRequest{Command: detectCanbusCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp obdAutoDetectResponse

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	canbusInfo = resp.CanbusInfo
	return
}

func GetVIN(unitID uuid.UUID) (vin, protocol string, err error) {
	for _, part := range getVinCommandParts() {
		// likely may not need bytes=20 or decode ascii
		cmd := fmt.Sprintf(`obd.query vin mode=%s pid=%s bytes=20 formula='%s.decode("ascii")' force=True protocol=%s`,
			part.Mode, part.PID, part.Formula, part.Protocol)

		req := executeRawRequest{Command: cmd}
		url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

		var resp executeRawResponse

		err = executeRequest("POST", url, req, &resp)
		if err != nil {
			continue // try again with different command if err
		}
		// if no error, we want to make sure we get a semblance of a vin back
		vin = resp.Value
		if validateVIN(vin) {
			return vin, part.Protocol, nil
		} else {
			err = fmt.Errorf("response contained an invalid vin: %s", vin)
		}
	}

	return
}

func ClearDiagnosticCodes(unitID uuid.UUID) (err error) {
	req := executeRawRequest{Command: clearDiagnosticCodeCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func GetDiagnosticCodes(unitID uuid.UUID) (codes string, err error) {
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

func validateVIN(vin string) bool {
	if len(vin) != 17 {
		return false
	}
	// match alpha numeric
	pattern := "[0-9A-Fa-f]+"
	regex := regexp.MustCompile(pattern)
	if !regex.MatchString(vin) {
		return false
	}
	// consider validating parts, eg. bring in shared vin library and then some basic validation for each part, or call out to service?

	return true
}

func getVinCommandParts() []vinCommandParts {
	return []vinCommandParts{
		{Formula: `'messages[0].data[3:20]'`, Protocol: "6", Header: "7DF", PID: "02", Mode: "09", VINCode: "vin_7DF_09_02"},
		{Formula: `'messages[0].data[3:20]'`, Protocol: "6", Header: "7e0", PID: "02", Mode: "09", VINCode: "vin_7e0_09_02"},
		{Formula: `'messages[0].data[3:20]'`, Protocol: "7", Header: "18DB33F1", PID: "02", Mode: "09", VINCode: "vin_18DB33F1_09_02"},
		{Formula: `'int(messages[0].data[4:],16)/10'`, Protocol: "6", Header: "7DF", PID: "A6", Mode: "01", VINCode: "vin_7DF_A6"},
		{Formula: `'int(messages[0].data[4:],16)/10'`, Protocol: "6", Header: "7DF", PID: "A6", Mode: "01", VINCode: "vin_7e0_A6"},
		{Formula: `'int(messages[0].data[4:],16)/10'`, Protocol: "7", Header: "18DB33F1", PID: "A6", Mode: "01", VINCode: "vin_18DB33F1_A6"},
		{Formula: `'messages[0].data[3:20]'`, Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS_3"},
		{Formula: `'messages[0].data[3:20]'`, Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS_3"},
		{Formula: `'messages[0].data[4:21]'`, Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS_4"},
		{Formula: `'messages[0].data[4:21]'`, Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS_4"},
		{Formula: `'messages[0].data[3:20]'`, Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS_3"},
		{Formula: `'messages[0].data[4:21]'`, Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS_4"},
		{Formula: `'messages[0].data[2:19]'`, Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS_2"},
		{Formula: `'messages[0].data[2:19]'`, Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS_2"},
		{Formula: `'messages[0].data[5:22]'`, Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS_5"},
		{Formula: `'messages[0].data[5:22]'`, Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS_2"},
		{Formula: `'messages[0].data[2:19]'`, Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS_2"},
		{Formula: `'messages[0].data[5:22]'`, Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS_5"},
	}
}

type vinCommandParts struct {
	Formula  string
	Protocol string
	Header   string
	PID      string
	Mode     string
	VINCode  string
}
