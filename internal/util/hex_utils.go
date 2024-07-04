package util

import (
	"fmt"
	"regexp"
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
