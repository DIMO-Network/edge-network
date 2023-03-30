package commands

import (
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"unicode"

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
		hdr := ""
		formula := ""
		if len(part.Header) > 0 {
			hdr = "header=" + part.Header
		}
		if len(part.Formula) > 0 {
			formula = fmt.Sprintf(`formula='%s.decode("ascii")'`, part.Formula)
		}
		cmd := fmt.Sprintf(`obd.query vin %s mode=%s pid=%s %s force=True protocol=%s`,
			hdr, part.Mode, part.PID, formula, part.Protocol)

		req := executeRawRequest{Command: cmd}
		url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

		var resp executeRawResponse

		err = executeRequest("POST", url, req, &resp)
		if err != nil {
			fmt.Println("failed to execute POST request to get vin: " + err.Error())
			continue // try again with different command if err
		}
		fmt.Println("received GetVIN response value: \n" + resp.Value) // for debugging - will want this to validate.
		// if no error, we want to make sure we get a semblance of a vin back
		if len(part.Formula) == 0 {
			// if no formula, means we got raw hex back so lets try extracting vin from that
			vin, err = extractVIN(resp.Value)
			if err != nil {
				fmt.Println("could not extract vin from hex: " + err.Error())
				continue // try again on next loop with different command
			}
		} else {
			vin = resp.Value
		}
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

func extractVIN(hexValue string) (vin string, err error) {
	// loop for each line, ignore what we don't want
	// start on the first 6th char,  cut out the first 5 of each line, convert that hex to ascii, remove any bad chars
	// use regexp to look for only good characters
	lines := strings.Split(hexValue, "\n")
	decodedVin := ""
	for _, line := range lines {
		if len(line) < 6 {
			continue
		}
		hx := line[5:] // remove start, why is this again, big endian vs little endian?
		// convert to ascii
		hexBytes, err := hex.DecodeString(hx)
		if err != nil {
			return "", err
		}
		asciiStr := ""
		for _, b := range hexBytes {
			asciiStr += string(b)
		}
		cleaned := ""
		// need to find start of clean character
		for pos, ch := range asciiStr {
			if unicode.IsUpper(ch) || unicode.IsDigit(ch) {
				cleaned = asciiStr[pos:]
				break
			}
		}

		if len(decodedVin) < 17 {
			decodedVin += cleaned
		}
	}
	return decodedVin, nil
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

// getVinCommandParts the PID command is composed of the protocol, header, PID and Mode. The Formula is just for
// software interpretation. If remove formula need to interpret it in software, will be raw hex
func getVinCommandParts() []vinCommandParts {
	return []vinCommandParts{
		// todo remove formula once can extract vin
		{Formula: `'messages[0].data[3:20]'`, Protocol: "6", Header: "7DF", PID: "02", Mode: "09", VINCode: "vin_7DF_09_02"},
		{Formula: `'messages[0].data[3:20]'`, Protocol: "6", Header: "7e0", PID: "02", Mode: "09", VINCode: "vin_7e0_09_02"},
		{Formula: `'messages[0].data[3:20]'`, Protocol: "7", Header: "18DB33F1", PID: "02", Mode: "09", VINCode: "vin_18DB33F1_09_02"},
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
