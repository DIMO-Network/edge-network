package loggers

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

//go:generate mockgen -source vin_logger.go -destination mocks/vin_logger_mock.go
type VINLogger interface {
	GetVIN(unitID uuid.UUID, queryName *string) (vinResp *VINResponse, err error)
}

type vinLogger struct {
}

func NewVINLogger() VINLogger {
	return &vinLogger{}
}

const citroenQueryName = "citroen"

func (vl *vinLogger) GetVIN(unitID uuid.UUID, queryName *string) (vinResp *VINResponse, err error) {
	// original vin command `obd.query vin mode=09 pid=02 bytes=20 formula='messages[0].data[3:].decode("ascii")' force=True protocol=auto`
	// protocol=auto means it just uses whatever bus is assigned to the autopi, but this is often incorrect so best to be explicit
	vin := ""
	for _, part := range getVinCommandParts() {
		if queryName != nil {
			if *queryName == citroenQueryName {
				return passiveScanCitroen()
			}
			if *queryName != part.QueryName {
				continue // skip until get to matching query
			}
		}
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

		req := api.ExecuteRawRequest{Command: cmd}
		url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

		var resp api.ExecuteRawResponse

		err = api.ExecuteRequest("POST", url, req, &resp)
		if err != nil {
			log.WithError(err).Error("failed to execute POST request to get vin")
			continue // try again with different command if err
		}
		log.Infof("received GetVIN response value: %s \n", resp.Value) // for debugging - will want this to validate.
		// if no error, we want to make sure we get a semblance of a vin back
		if len(part.Formula) == 0 {
			// if no formula, means we got raw hex back so lets try extracting vin from that
			vin, _, err = extractVIN(resp.Value) // todo: do something with the pid vin start position - persist for later to backend
			if err != nil {
				log.WithError(err).Error("could not extract vin from hex")
				continue // try again on next loop with different command
			}
		} else {
			vin = resp.Value
		}
		if validateVIN(vin) {
			return &VINResponse{
				VIN:       vin,
				Protocol:  part.Protocol,
				QueryName: part.QueryName,
			}, nil
		}
		err = fmt.Errorf("response contained an invalid vin: %s", vin)
	}

	// if all PIDs fail, try passive scan, currently specific to Citroen
	if err != nil {
		vinResp, _ = passiveScanCitroen()
	}
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
	if !validateVIN(decodedVin) {
		return "", 0, fmt.Errorf("could not extract a valid VIN, result was: %s", decodedVin)
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
	return regex.MatchString(vin)
}

func isEven(num int) bool {
	return num%2 == 0
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
	if len(contentLines) > 0 {
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
	}

	return pos - 1
}

func passiveScanCitroen() (*VINResponse, error) {
	vin := ""
	// unable to get VIN via PID queries, so try passive scan
	myVinReader := newPassiveVinReader()
	protocolInt := 0

	// Create a channel to receive the result
	resultChan := make(chan string)
	// Create a channel for the timeout signal
	timeoutChan := make(chan bool)
	go func() {
		vin, protocolInt, _ = myVinReader.ReadCitroenVIN(10000) // todo cleanup logging in this method
		resultChan <- vin
	}()
	// Start a goroutine to wait for the timeout
	go func() {
		time.Sleep(10 * time.Second) // Set your desired timeout duration
		timeoutChan <- true
	}()

	// Wait for either the function result or the timeout signal
	select {
	case vinResult := <-resultChan:
		log.Infof("Citroen VIN scan completed within timeout: %s", vinResult)
		if validateVIN(vin) {
			return &VINResponse{
				VIN:       vinResult,
				Protocol:  strconv.Itoa(protocolInt),
				QueryName: citroenQueryName,
			}, nil
		}
	case <-timeoutChan:
		err := fmt.Errorf("citroen VIN scan timed out")
		return nil, err
	}
	return nil, fmt.Errorf("could not get citroen vin")
}

// getVinCommandParts the PID command is composed of the protocol, header, PID and Mode. The Formula is just for
// software interpretation. If remove formula need to interpret it in software, will be raw hex
func getVinCommandParts() []vinCommandParts {
	return []vinCommandParts{
		{Protocol: "6", Header: "7DF", PID: "02", Mode: "09", QueryName: "vin_7DF_09_02"},
		{Protocol: "6", Header: "7e0", PID: "02", Mode: "09", QueryName: "vin_7e0_09_02"},
		{Protocol: "7", Header: "18DB33F1", PID: "02", Mode: "09", QueryName: "vin_18DB33F1_09_02"},
		{Protocol: "6", Header: "7df", PID: "F190", Mode: "22", QueryName: "vin_7DF_UDS"},
		{Protocol: "6", Header: "7e0", PID: "F190", Mode: "22", QueryName: "vin_7e0_UDS"},
		{Protocol: "7", Header: "18DB33F1", PID: "F190", Mode: "22", QueryName: "vin_18DB33F1_UDS"},
		{QueryName: "citroen"},
	}
}

type vinCommandParts struct {
	Formula   string
	Protocol  string
	Header    string
	PID       string
	Mode      string
	QueryName string
}

type VINResponse struct {
	VIN       string
	Protocol  string
	QueryName string
}
