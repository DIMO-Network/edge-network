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
			vin, _, err = extractVIN(resp.Value) // todo: do something with the pid vin start position - persist for later to backend
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

func extractVIN(hexValue string) (vin string, startPosition int, err error) {
	// loop for each line, ignore what we don't want
	// start on the first 6th char,  cut out the first 5 of each line, convert that hex to ascii, remove any bad chars
	// use regexp to look for only good characters
	lines := strings.Split(hexValue, "\n")
	cutStartPos := findVINLineStart(lines)
	decodedVin := ""
	for _, line := range lines {
		if len(line) < 6 {
			continue
		}
		hx := line[cutStartPos:] // remove start, why is this again, big endian vs little endian? Protocol 7 may be different
		if !isEven(len(hx)) {
			hx = hx[1:] // cut one more if we get an odd length
		}
		// convert to ascii
		hexBytes, err := hex.DecodeString(hx)
		if err != nil {
			return "", 0, err
		}
		asciiStr := ""
		for _, b := range hexBytes {
			asciiStr += string(b)
		}
		cleaned := ""
		// need to find start of clean character
		for pos, ch := range asciiStr {
			// todo investigate: is the byte being expanded to eg. \x02 instead of a single unicode character, thus blowing up the length?
			// todo: subaru example payload had good example of \x02\x01 as ascii but in short is just a single unicode char.
			if unicode.IsUpper(ch) || unicode.IsDigit(ch) {
				cleaned = asciiStr[pos:]
				if len(decodedVin) == 0 {
					startPosition = pos // store the vin start position so we know when setting up pid logger
				}
				break
			}
		}

		if len(decodedVin) < 17 {
			decodedVin += cleaned
		}
	}
	strLen := len(decodedVin)
	if strLen > 17 {
		startPosition = startPosition + (strLen - 17)
		decodedVin = decodedVin[strLen-17:]
	}
	return decodedVin, startPosition - 4, nil // subtract 4 from start position to make up for random crap, not sure how this will work
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

func isEven(num int) bool {
	if num%2 == 0 {
		return true
	}
	return false
}

func findVINLineStart(lines []string) int {
	const defaultPosition = 5
	pos := defaultPosition
	var contentLines []string
	// remove lines that aren't core part
	for _, line := range lines {
		if len(line) < 5 {
			continue
		}
		contentLines = append(contentLines, line)
	}
	// for each character on the first line, up to what position do the rest of lines have the same characters in order.
	for i, ch := range contentLines[0] {
		for _, line2 := range contentLines[1:] {
			if ch != int32(line2[i]) {
				pos = i
				break
			}
		}
		if pos != defaultPosition {
			break
		}
	}

	return pos - 1
}

// getVinCommandParts the PID command is composed of the protocol, header, PID and Mode. The Formula is just for
// software interpretation. If remove formula need to interpret it in software, will be raw hex
func getVinCommandParts() []vinCommandParts {
	return []vinCommandParts{
		{Protocol: "6", Header: "7DF", PID: "02", Mode: "09", VINCode: "vin_7DF_09_02"},
		{Protocol: "6", Header: "7e0", PID: "02", Mode: "09", VINCode: "vin_7e0_09_02"},
		{Protocol: "7", Header: "18DB33F1", PID: "02", Mode: "09", VINCode: "vin_18DB33F1_09_02"},
		{Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS"},
		{Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS"},
		//{Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS_4"},
		//{Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS_4"},
		{Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS"},
		//{Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS_4"},
		//{Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS_2"},
		//{Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS_2"},
		//{Protocol: "6", Header: "7df", PID: "F190", Mode: "22", VINCode: "vin_7DF_UDS_5"},
		//{Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", VINCode: "vin_7e0_UDS_2"},
		//{Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS_2"},
		//{Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", VINCode: "vin_18DB33F1_UDS_5"},
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
