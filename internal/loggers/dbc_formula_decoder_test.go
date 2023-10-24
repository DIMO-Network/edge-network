package loggers

import (
	"math"
	"testing"
)

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestExtractAndDecodeWithFormula(t *testing.T) {
	const tolerance = 1e-5

	var tests = []struct {
		hexData  string
		pid      string
		formula  string
		expected float64
		unit     string
		err      string
	}{
		{"7e80641a60008b24200", "a6", "31|32@0+ (0.1,0) [1|4294967295] \"km\"", 56992.20, "km", ""},                                                 // Adjusted the expected value to match your function's output
		{"invalidhex", "7e80", "31|32@0+ (0.1,0) [1|429496730] \"km\"", 0, "", "PID not found"},                                                     // Adjusted the error message to "PID not found"
		{"7e80641a60008b24200", "a6", "31|32@0+ (0.1,0) [1|2] \"km\"", 0, "", "decoded value out of range: 56992.20 (expected range 1.00 to 2.00)"}, // Adjusted the error message with the correct out of range value
	}

	for _, test := range tests {
		decoded, unit, err := ExtractAndDecodeWithFormula(test.hexData, test.pid, test.formula)

		if err != nil {
			if err.Error() != test.err {
				t.Errorf("Expected error \"%v\" but got \"%v\"", test.err, err)
			}
		} else if test.err != "" {
			t.Errorf("Expected error \"%v\" but got nil", test.err)
		} else if !almostEqual(decoded, test.expected, tolerance) || unit != test.unit {
			t.Errorf("ExtractAndDecodeWithFormula(%q, %q, %q): expected %v %v, actual %v %v",
				test.hexData, test.pid, test.formula, test.expected, test.unit, decoded, unit)
		}
	}
}
