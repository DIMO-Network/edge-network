package loggers

import (
	"math"
	"testing"
)

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestExtractAndDecodeWithFormula(t *testing.T) {
	const tolerance = 1e-2

	var tests = []struct {
		hexData  string
		pid      string
		formula  string
		expected float64
		unit     string
		err      string
	}{
		//odometer
		{"7e80641a60008b24200", "a6", "31|32@0+ (0.1,0) [1|4294967295] \"km\"", 56992.20, "km", ""},
		{"invalidhex", "7e80", "31|32@0+ (0.1,0) [1|429496730] \"km\"", 0, "", "PID not found"},
		{"7e80641a60008b24200", "a6", "31|32@0+ (0.1,0) [1|2] \"km\"", 0, "", "decoded value out of range: 56992.20 (expected range 1.00 to 2.00)"},
		{"7e80641a60001f2adcc", "a6", "31|32@0+ (0.1,0) [1|4294967295] \"km\"", 12766.10, "km", ""},
		{"7e80641a600091d0d00", "a6", "31|32@0+ (0.1,0) [1|4294967295] \"km\"", 59726.10, "km", ""},
		//fuel level
		{"7e803412f6700000000", "2f", "31|8@0+ (0.392156862745098,0) [0|100] \"%\"", 40.39, "%", ""},
		{"7e803412f26cccccccc", "2f", "31|8@0+ (0.392156862745098,0) [0|100] \"%\"", 14.9, "%", ""},
		//coolant temp
		{"7e803410581cccccccc", "5", "31|8@0+ (1,-40) [-40|215] \"degC\"", 89, "degC", ""},
		{"7e803410585aaaaaaaa", "5", "31|8@0+ (1,-40) [-40|215] \"degC\"", 93, "degC", ""},
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
