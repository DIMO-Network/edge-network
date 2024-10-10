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

// ForceFirstTwoBytesAndSwapLast forces the first two bytes of the input value to be 0x18da.
// It then masks out the last two bytes and swaps them.
// Finally, it combines the forced first two bytes with the swapped last two bytes and returns the result.
func ForceFirstTwoBytesAndSwapLast(val uint32) uint32 {
	// Force the first two bytes to be 0x18da
	forcedFirstTwoBytes := uint32(0x18da0000)

	// Mask out the last two bytes
	lastTwoBytes := val & 0xFFFF // Extract the last two bytes (e.g., 0x33f1)

	// Swap the last two bytes
	swappedBytes := (lastTwoBytes >> 8) | (lastTwoBytes<<8)&0xFFFF // Swap 0x33 and 0xf1

	// Combine forced first two bytes with swapped last two bytes
	return forcedFirstTwoBytes | swappedBytes
}
