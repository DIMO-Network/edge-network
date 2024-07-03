package loggers

import (
	"encoding/hex"
	"errors"
	"fmt"
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

// ParsePIDBytesWithDBCFormula same as above but only parses out float values and uses native bytes for data and uint for pid
// hexData does not include header / frame ID, if pid is -1 it is not considered
func ParsePIDBytesWithDBCFormula(frameData []byte, pid uint32, formula string) (float64, string, error) {
	formula = strings.TrimPrefix(formula, "dbc:")
	// Parse formula
	re := regexp.MustCompile(`(\d+)\|(\d+)@(\d+)\+ \(([^,]+),([^)]+)\) \[([^|]+)\|([^]]+)] "([^"]+)"`)
	matches := re.FindStringSubmatch(formula)

	if len(matches) != 9 {
		return 0, "", fmt.Errorf("invalid formula format: %s", formula)
	}
	fmt.Printf("\nframe data: %s\n", printBytesAsHex(frameData)) // debug

	// not sure if need this since the response contains the data length
	//lengthBits, err := strconv.Atoi(matches[2])
	//if err != nil {
	//	return 0, "", err
	//}
	//numBytes := lengthBits / 8

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
	dataLength := frameData[0]
	dataEndPos := pidIndex + uint32(dataLength) - 1
	if len(frameData) < int(dataLength) {
		dataEndPos = uint32(len(frameData))
		fmt.Printf("used the frame data length: %d \n", dataEndPos) // debug
	}

	// Extract the relevant portion of the hex data
	valueBytes := frameData[pidIndex+1 : dataEndPos]
	fmt.Printf("value bytes: %s\n", printBytesAsHex(valueBytes)) // debug

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
