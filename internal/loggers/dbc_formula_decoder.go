package loggers

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ExtractAndDecodeWithDBCFormula extracts the data following the PID and applies the formula for decoding
func ExtractAndDecodeWithDBCFormula(hexData, pid, formula string) (float64, string, error) {
	formula = strings.TrimPrefix(formula, "dbc:")
	// Parse formula
	re := regexp.MustCompile(`(\d+)\|(\d+)@(\d+)\+ \(([^,]+),([^)]+)\) \[([^|]+)\|([^]]+)] "([^"]+)"`)
	matches := re.FindStringSubmatch(formula)

	if len(matches) != 9 {
		return 0, "", fmt.Errorf("invalid formula format: %s", formula)
	}

	lengthBits, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, "", err
	}
	numBytes := lengthBits / 8

	slicedHexData := hexData
	if len(hexData) > 7 {
		slicedHexData = hexData[7:]
	}

	// Find the index of PID in the sliced hex string
	pidIndex := strings.Index(strings.ToLower(slicedHexData), strings.ToLower(pid))
	if pidIndex == -1 {
		// todo - is this always the case that the PID will be returned in resp?
		return 0, "", errors.New("PID not found")
	}

	// Adjust pidIndex to account for the initially sliced part
	totalPidIndex := pidIndex + 7
	if totalPidIndex+numBytes*2 > len(hexData) {
		return 0, "", errors.New("not enough data")
	}

	// Extract the relevant portion of the hex data
	valueHex := hexData[totalPidIndex+len(pid) : totalPidIndex+len(pid)+numBytes*2]

	value, err := strconv.ParseUint(valueHex, 16, 64)
	if err != nil {
		return 0, "", err
	}

	// Parse the formula parameters
	scaleFactor, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return 0, "", err
	}
	offsetAdjustment, err := strconv.ParseFloat(matches[5], 64)
	if err != nil {
		return 0, "", err
	}
	minValue, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return 0, "", err
	}
	maxValue, err := strconv.ParseFloat(matches[7], 64)
	if err != nil {
		return 0, "", err
	}
	unit := matches[8]

	// Apply the formula
	decodedValue := float64(value)*scaleFactor + offsetAdjustment

	// Validate the range
	if decodedValue < minValue || decodedValue > maxValue {
		return 0, "", fmt.Errorf("decoded value out of range: %.2f (expected range %.2f to %.2f)", decodedValue, minValue, maxValue)
	}

	return decodedValue, unit, nil
}

// ParsePIDBytesWithDBCFormula same as above but meant for parsing PID or DID responses.
// hexData does not include header / frame ID, pid is the expected pid we're looking for in response
func ParsePIDBytesWithDBCFormula(frameData []byte, pid uint32, formula string) (float64, string, error) {
	formula = strings.TrimPrefix(formula, "dbc:")
	// Parse formula
	re := regexp.MustCompile(`(\d+)\|(\d+)@(\d+)\+ \(([^,]+),([^)]+)\) \[([^|]+)\|([^]]+)] "([^"]+)"`)
	matches := re.FindStringSubmatch(formula)

	if len(matches) != 9 {
		return 0, "", fmt.Errorf("invalid formula format: %s", formula)
	}

	// Find the index of PID in the byte array
	pidIndex := uint32(0)
	if pid > 0 {
		byteVal := byte(pid)
		for i, v := range frameData {
			if v == byteVal {
				pidIndex = uint32(i)
				break
			}
		}
	}
	if pidIndex == 0 {
		return 0, "", fmt.Errorf("PID %d not found in response frameData: %s", pid, printBytesAsHex(frameData))
	}

	lengthBits, err := strconv.Atoi(matches[2]) // eg. get the 8 in `31|8 ...`
	if err != nil {
		return 0, "", err
	}
	numBytes := lengthBits / 8

	//dataLength := frameData[0] // if we wanted to get length based off the pid response, instead of the formula
	dataEndPos := pidIndex + uint32(numBytes) + 1 // use the length from formula
	if len(frameData) < int(dataEndPos) {
		// control for formula data length being longer than available bytes
		dataEndPos = uint32(len(frameData))
	}

	// Extract the relevant portion of the hex data
	valueBytes := frameData[pidIndex+1 : dataEndPos]
	//fmt.Printf("value bytes: %s\n", printBytesAsHex(valueBytes)) // debug

	// ideally here we'd used binary.LittleEnding.Uint64 or BigEndian per formula, pad the array prior
	valueHex := hex.EncodeToString(valueBytes)
	value, err := strconv.ParseUint(valueHex, 16, 64)
	if err != nil {
		return 0, "", err
	}

	// Parse the formula parameters
	scaleFactor, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return 0, "", err
	}
	offsetAdjustment, err := strconv.ParseFloat(matches[5], 64)
	if err != nil {
		return 0, "", err
	}
	minValue, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return 0, "", err
	}
	maxValue, err := strconv.ParseFloat(matches[7], 64)
	if err != nil {
		return 0, "", err
	}
	unit := matches[8]

	// Apply the formula
	decodedValue := float64(value)*scaleFactor + offsetAdjustment

	// Validate the range
	if decodedValue < minValue || decodedValue > maxValue {
		return 0, "", fmt.Errorf("decoded value out of range: %.2f (expected range %.2f to %.2f)", decodedValue, minValue, maxValue)
	}

	return decodedValue, unit, nil
}

// DecodePassiveFrame takes in passively captured frame via DBC file that is meant to be decoded with a DBC formula.
// Used for DBC passive scanning decoding values.
func DecodePassiveFrame(frameData []byte, dbcFormula string) (float64, string, error) {
	// Parse formula
	re := regexp.MustCompile(`(\d+)\|(\d+)@(\d+)\+ \(([^,]+),([^)]+)\) \[([^|]+)\|([^]]+)] "([^"]+)"`)
	matches := re.FindStringSubmatch(dbcFormula)

	if len(matches) != 9 {
		return 0, "", fmt.Errorf("invalid formula format: %s", dbcFormula)
	}

	lengthBits, err := strconv.Atoi(matches[2]) // eg. get the 24 in `7|24 ...`
	if err != nil {
		return 0, "", err
	}
	numBytes := lengthBits / 8

	startPosBits, err := strconv.Atoi(matches[1]) // eg. get the 7 in `7|24 ...`
	if err != nil {
		return 0, "", err
	}
	// this part I don't get, starting on pos bit 7 is like starting at the beginning of the data frame as the socket library gets it.
	if startPosBits >= 7 {
		startPosBits = startPosBits - 7
	}
	// is odd, not sure if need this
	if startPosBits%2 != 0 {
		startPosBits++
	}

	startBytes := startPosBits / 8

	dataEndPos := startBytes + numBytes // +1

	if len(frameData) < dataEndPos {
		// control for formula data length being longer than available bytes
		return 0, "", fmt.Errorf("formula length longer than frame data length: %d vs. %d", dataEndPos, len(frameData))
	}

	valueBytes := frameData[startBytes:dataEndPos]
	valueHex := hex.EncodeToString(valueBytes)
	//fmt.Printf("value in hex: %s\n", valueHex) // for debugging
	value, err := strconv.ParseUint(valueHex, 16, 64)
	if err != nil {
		return 0, "", err
	}
	// Parse the formula parameters
	scaleFactor, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return 0, "", err
	}

	offsetAdjustment, err := strconv.ParseFloat(matches[5], 64)
	if err != nil {
		return 0, "", err
	}
	minValue, err := strconv.ParseFloat(matches[6], 64)
	if err != nil {
		return 0, "", err
	}
	maxValue, err := strconv.ParseFloat(matches[7], 64)
	if err != nil {
		return 0, "", err
	}
	unit := matches[8]

	// Apply the formula
	decodedValue := float64(value)*scaleFactor + offsetAdjustment

	// Validate the range
	if decodedValue < minValue || decodedValue > maxValue {
		return 0, "", fmt.Errorf("decoded value out of range: %.2f (expected range %.2f to %.2f)", decodedValue, minValue, maxValue)
	}

	return roundToTwoDecimals(decodedValue), unit, nil
}

func roundToTwoDecimals(f float64) float64 {
	return math.Round(f*100) / 100
}
