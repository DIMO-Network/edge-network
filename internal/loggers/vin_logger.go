package loggers

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"
	"unicode"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/models"

	"github.com/rs/zerolog"

	"github.com/google/uuid"
)

//go:generate mockgen -source vin_logger.go -destination mocks/vin_logger_mock.go
type VINLogger interface {
	GetVIN(unitID uuid.UUID, queryName *string) (vinResp *VINResponse, err error)
}

type vinLogger struct {
	mu     sync.Mutex
	logger zerolog.Logger
}

func NewVINLogger(logger zerolog.Logger) VINLogger {
	return &vinLogger{logger: logger}
}

const VINLoggerVersion = 1 // increment this if improve support for decoding VINs
const citroenQueryName = "citroen"

// GetVIN gets the vin through a variety of methods. If a queryName is passed in, it uses the specific named method to get VIN
func (vl *vinLogger) GetVIN(unitID uuid.UUID, queryName *string) (vinResp *VINResponse, err error) {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	// original vin command `obd.query vin mode=09 pid=02 bytes=20 formula='messages[0].data[3:].decode("ascii")' force=True protocol=auto`
	// protocol=auto means it just uses whatever bus is assigned to the autopi, but this is often incorrect so best to be explicit
	vin := ""
	for _, part := range getVinCommandParts() {
		if queryName != nil {
			if *queryName == citroenQueryName {
				return passiveScanCitroen(vl.logger)
			}
			if *queryName != part.Name {
				continue // skip until get to matching query
			}
		}
		resp, _, vinErr := commands.RequestPIDRaw(&vl.logger, unitID, part)
		if vinErr != nil {
			vl.logger.Err(vinErr).Msgf("query %s failed to get vin", part.Name)
			continue
		}
		vl.logger.Debug().Msgf("received GetVIN response: %+v", resp)

		// try to extract the VIN from the raw hex string
		if resp.IsHex {
			// if no formula, means we got raw hex back so lets try extracting vin from that
			vin, _, err = extractVIN(resp.ValueHex)
			if err != nil {
				vl.logger.Err(err).Msg("could not extract vin from hex")
				continue // try again on next loop with different command
			}
		} else {
			vin = resp.Value.(string)
		}
		if validateVIN(vin) {
			return &VINResponse{
				VIN:       vin,
				Protocol:  part.Protocol,
				QueryName: part.Name,
			}, nil
		}
		// just set the err, don't return it in the loop since we want to try all options
		err = fmt.Errorf("response contained an invalid vin: %s", vin)
	}

	// if all PIDs fail, try passive scan, currently specific to Citroen
	if err != nil {
		vinResp, _ = passiveScanCitroen(vl.logger)
		if vinResp != nil {
			err = nil
		}
	}
	if vinResp == nil {
		err = fmt.Errorf("unable to get VIN with any method")
	}
	return
}

// extractVIN converts the raw hex (as string) into a VIN by some algorithms
func extractVIN(hexValueFrames []string) (vin string, startPosition int, err error) {
	// loop for each line, ignore what we don't want
	// start on the first 6th char,  cut out the first 5 of each line, convert that hex to ascii, remove any bad chars
	// use regexp to look for only good characters
	cutStartPos := findVINLineStart(hexValueFrames)
	decodedVin := ""
	for _, line := range hexValueFrames {
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
	//nolint
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

func passiveScanCitroen(logger zerolog.Logger) (*VINResponse, error) {
	vin := ""
	// unable to get VIN via PID queries, so try passive scan
	vinReader := newPassiveVinReader()
	protocolInt := 0

	// Create a channel to receive the result
	resultChan := make(chan string)
	// Create a channel for the timeout signal
	timeoutChan := make(chan bool)
	go func() {
		vin, protocolInt, _ = vinReader.ReadCitroenVIN(10000) // todo cleanup logging in this method
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
		logger.Info().Msgf("Citroen VIN scan completed within timeout: %s", vinResult)
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
func getVinCommandParts() []models.PIDRequest {
	return []models.PIDRequest{
		{Protocol: "6", Header: 2015, Pid: 2, Mode: 9, Name: "vin_7DF_09_02"},
		{Protocol: "6", Header: 2016, Pid: 2, Mode: 9, Name: "vin_7e0_09_02"},
		{Protocol: "7", Header: 417018865, Pid: 2, Mode: 9, Name: "vin_18DB33F1_09_02"},
		{Protocol: "6", Header: 2015, Pid: 61840, Mode: 34, Name: "vin_7DF_UDS"},
		{Protocol: "6", Header: 2016, Pid: 61840, Mode: 34, Name: "vin_7e0_UDS"},
		{Protocol: "7", Header: 417018865, Pid: 61840, Mode: 34, Name: "vin_18DB33F1_UDS"},
		{Name: "citroen"},
	}
}

type VINResponse struct {
	VIN       string
	Protocol  string
	QueryName string
}
