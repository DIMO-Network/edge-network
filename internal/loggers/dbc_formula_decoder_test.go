package loggers

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
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
		{"7e803410585aaaaaaaa", "05", "31|8@0+ (1,-40) [-40|215] \"degC\"", 93, "degC", ""}, // 0 padded pid
		// mache odoemteter
		{"7e80662dd01003f5acc", "dd01", "39|24@0+ (1,0) [0|2150000] \"km\"", 16218, "km", ""},
		// gm120 dbc passive odo - todo: need real example data
		{"7e800000000dd3f5acc", "", "7|32@0+ (0.015625,0) [0|67108863.984375] \"km\" Vector_XXX", 884.984375, "km", ""},
	}

	for _, test := range tests {
		decoded, unit, err := ExtractAndDecodeWithDBCFormula(test.hexData, test.pid, test.formula)

		if err != nil {
			if err.Error() != test.err {
				t.Errorf("Expected error \"%v\" but got \"%v\"", test.err, err)
			}
		} else if test.err != "" {
			t.Errorf("Expected error \"%v\" but got nil", test.err)
		} else if !almostEqual(decoded, test.expected, tolerance) || unit != test.unit {
			t.Errorf("ExtractAndDecodeWithDBCFormula(%q, %q, %q): expected %v %v, actual %v %v",
				test.hexData, test.pid, test.formula, test.expected, test.unit, decoded, unit)
		}
	}
}

func TestParseBytesWithDBCFormula(t *testing.T) {
	type args struct {
		frameData []byte
		pid       uint32
		formula   string
	}
	tests := []struct {
		name      string
		args      args
		wantValue float64
		wantUnit  string
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name: "rpm",
			args: args{
				frameData: hexToByteArray("04 41 0C 0F FE", t),
				pid:       uint32(12),
				formula:   `31|16@0+ (0.25,0) [0|16383.75] "rpm"`,
			},
			wantValue: 1023.5,
			wantUnit:  "rpm",
			wantErr:   assert.NoError,
		},
		{
			name: "barometricPressure",
			args: args{
				frameData: hexToByteArray("03 41 33 65", t),
				pid:       uint32(51), //x33
				formula:   `31|8@0+ (1,0) [0|255] "kPa"`,
			},
			wantValue: 101,
			wantUnit:  "kPa",
			wantErr:   assert.NoError,
		},
		{
			name: "longFuelTrim",
			args: args{
				frameData: hexToByteArray("03 41 07 A0", t),
				pid:       uint32(7), //x07
				formula:   `31|8@0+ (0.78125,-100) [-100|99.21875] "%"`,
			},
			wantValue: 25,
			wantUnit:  "%",
			wantErr:   assert.NoError,
		},
		{
			name: "warmupsSinceDtccCear",
			args: args{
				frameData: hexToByteArray("22 33 30 32 35 33 31 31", t),
				pid:       uint32(48), //0x30
				formula:   `31|8@0+ (1,0) [0|255] "count"`,
			},
			wantValue: 50,
			wantUnit:  "count",
			wantErr:   assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, unit, err := ParsePIDBytesWithDBCFormula(tt.args.frameData, tt.args.pid, tt.args.formula)
			if !tt.wantErr(t, err, fmt.Sprintf("ParsePIDBytesWithDBCFormula(%v, %v, %v)", tt.args.frameData, tt.args.pid, tt.args.formula)) {
				return
			}
			assert.Equalf(t, tt.wantValue, value, "ParsePIDBytesWithDBCFormula(%v, %v, %v)", tt.args.frameData, tt.args.pid, tt.args.formula)
			assert.Equalf(t, tt.wantUnit, unit, "ParsePIDBytesWithDBCFormula(%v, %v, %v)", tt.args.frameData, tt.args.pid, tt.args.formula)
		})
	}
}
