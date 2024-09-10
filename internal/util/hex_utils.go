package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// IsValidHex checks if the input string is a valid hexadecimal.
func IsValidHex(s string) bool {
	// Regex to match a valid hexadecimal string.
	// It starts with an optional "0x" or "0X", followed by one or more hexadecimal characters (0-9, a-f, A-F).
	re := regexp.MustCompile(`^(0x|0X)?[0-9a-fA-F]+$`)
	return re.MatchString(s)
}

// IsHexFrames checks if the input string consists of valid hexadecimal strings separated by newline characters.
func IsHexFrames(s string) bool {
	frames := strings.Split(s, "\n")
	for _, frame := range frames {
		if frame == "|-" { // autopi specific resp thing for multiframes
			continue
		}
		if !IsValidHex(frame) {
			return false
		}
	}
	return true
}

// UintToHexStr converts the uint32 into a 0 padded hex representation, always assuming must be even length.
func UintToHexStr(val uint32) string {
	hexStr := fmt.Sprintf("%X", val)
	if len(hexStr)%2 != 0 {
		return "0" + hexStr // Prepend a "0" if the length is odd
	}
	return hexStr
}

// HexToDecimal takes a hex string and converts it to a uint32 decimal representation.
func HexToDecimal(hexStr string) (uint32, error) {
	// Use strconv to parse the hex string (base 16) to an integer
	result, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return 0, err
	}

	return uint32(result), nil
}

func SwapLastTwoBytes(hexStr string) (string, error) {
	// Check if the hex string length is valid (must be even and at least 4 characters)
	if len(hexStr) < 4 || len(hexStr)%2 != 0 {
		return "", fmt.Errorf("invalid hex string")
	}

	// Split the hex string into two parts
	prefix := hexStr[:len(hexStr)-4]       // Everything except the last two bytes
	lastTwoBytes := hexStr[len(hexStr)-4:] // The last two bytes (4 hex chars)

	// Swap the last two bytes
	swappedLastTwoBytes := lastTwoBytes[2:] + lastTwoBytes[:2]

	// Return the result by concatenating the prefix with the swapped bytes
	return prefix + strings.ToUpper(swappedLastTwoBytes), nil
}
