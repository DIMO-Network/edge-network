package loggers

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/DIMO-Network/edge-network/internal/canbus"
	"github.com/DIMO-Network/edge-network/internal/models"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

//go:embed test_gm120.dbc
var testgm120dbc string

//go:embed test_acura_ilx.dbc
var testacurailxdbc string

func Test_dbcPassiveLogger_parseDBCHeaders(t *testing.T) {
	testLogger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})

	tests := []struct {
		name    string
		dbcFile string
		want    []dbcFilter
	}{
		{
			name:    "gm odometer -single header",
			dbcFile: testgm120dbc,
			want: []dbcFilter{
				{
					header: 288,
					signals: []dbcSignal{
						{
							formula:    `7|32@0+ (0.015625,0) [0|67108863.984375] "km" Vector_XXX`,
							signalName: "odometer",
						},
					},
				},
			},
		},
		{
			name:    "acura ilx - multiple headers",
			dbcFile: testacurailxdbc,
			want: []dbcFilter{
				{
					header: 304,
					signals: []dbcSignal{
						{
							formula:    `7|16@0- (1,0) [-1000|1000] "Nm" EON`,
							signalName: "ENGINE_TORQUE_ESTIMATE",
						},
						{
							formula:    `23|16@0- (1,0) [-1000|1000] "Nm" EON`,
							signalName: "ENGINE_TORQUE_REQUEST",
						},
						{
							formula:    `39|8@0+ (1,0) [0|255] "" EON`,
							signalName: "CAR_GAS",
						},
					},
				},
				{
					header: 316,
					signals: []dbcSignal{
						{
							formula:    `39|8@0+ (1,0) [0|255] "" EON`,
							signalName: "CAR_GAS",
						},
						{
							formula:    `61|2@0+ (1,0) [0|3] "" EON`,
							signalName: "COUNTER",
						},
					},
				},
				{
					header: 344,
					signals: []dbcSignal{
						{
							formula:    `7|16@0+ (0.01,0) [0|250] "kph" EON`,
							signalName: "XMISSION_SPEED",
						},
						{
							formula:    `23|16@0+ (1,0) [0|15000] "rpm" EON`,
							signalName: "ENGINE_RPM",
						},
						{
							formula:    `39|16@0+ (0.01,0) [0|250] "kph" EON`,
							signalName: "XMISSION_SPEED2",
						},
						{
							formula:    `55|8@0+ (10,0) [0|2550] "m" XXX`,
							signalName: "ODOMETER",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dpl := &dbcPassiveLogger{
				logger: testLogger,
			}
			parsed, err := dpl.parseDBCHeaders(tt.dbcFile)
			require.NoError(t, err)
			require.Len(t, parsed, len(tt.want))

			for i := range tt.want {
				assert.Equal(t, tt.want[i], parsed[i])
			}
		})
	}
}

// sample coolant request: 7df# 02 01 05 00 00 00 00 00
// sample coolant response: 7e8# 03 41 05 53
// - 03: Number of additional data bytes, ie. 41,05,03
// - 41: Response to service 01
// - 05: coolant temp PID
// - 53: coolant data - 83 - 40 = 43
func Test_dbcPassiveLogger_matchPID(t *testing.T) {
	testLogger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})
	pids := []models.PIDRequest{
		{
			Formula:         "dbc: 31|8@0+ (1,-40) [-40|215]",
			Header:          2015, // 7df
			IntervalSeconds: 60,
			Mode:            1, // 01
			Name:            "coolantTemp",
			Pid:             5, // 05
			Protocol:        "CAN11_500",
			ResponseHeader:  2024,
		},
	}

	dpl := &dbcPassiveLogger{
		logger:          testLogger,
		dbcFile:         nil,
		hardwareSupport: true,
		pids:            pids,
	}

	tests := []struct {
		name        string
		frame       canbus.Frame
		wantPIDName string
	}{
		{
			name: "match coolant temp",
			frame: canbus.Frame{
				ID:   2024,
				Data: hexToByteArray("03 41 05 53", t),
			},
			wantPIDName: "coolantTemp",
		},
		// todo test for extended frame
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := dpl.matchPID(tt.frame)
			fmt.Printf("%+v\n", match)
			if tt.wantPIDName != "" {
				assert.Equalf(t, tt.wantPIDName, match.Name, "matchPID(%v)", tt.frame)
			} else {
				assert.Nil(t, match, "expected nil match")
			}
		})
	}
}

func hexToByteArray(hexString string, t *testing.T) []byte {
	cleanHex := strings.Replace(hexString, " ", "", -1)
	byteArray, err := hex.DecodeString(cleanHex)
	if err != nil {
		t.Fatal(err)
	}

	return byteArray
}

func TestParseUniqueResponseHeaders(t *testing.T) {
	tests := []struct {
		name string
		pids []models.PIDRequest
		want map[uint32]struct{}
	}{
		{
			name: "No PID",
			pids: []models.PIDRequest{},
			want: map[uint32]struct{}{},
		},
		{
			name: "Single PID",
			pids: []models.PIDRequest{
				{
					ResponseHeader: 0x123,
				},
			},
			want: map[uint32]struct{}{
				0x123: {},
			},
		},
		{
			name: "Multiple Unique PIDs",
			pids: []models.PIDRequest{
				{
					ResponseHeader: 0x123,
				},
				{
					ResponseHeader: 0x456,
				},
				{
					ResponseHeader: 0x789,
				},
			},
			want: map[uint32]struct{}{
				0x123: {},
				0x456: {},
				0x789: {},
			},
		},
		{
			name: "Multiple Duplicate PIDs",
			pids: []models.PIDRequest{
				{
					ResponseHeader: 0x123,
				},
				{
					ResponseHeader: 0x123,
				},
				{
					ResponseHeader: 0x123,
				},
			},
			want: map[uint32]struct{}{
				0x123: {},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseUniqueResponseHeaders(tc.pids); !compareMaps(got, tc.want) {
				t.Errorf("parseUniqueResponseHeaders() = %v, want %v", got, tc.want)
			}
		})
	}
}

func compareMaps(a, b map[uint32]struct{}) bool {
	if len(a) != len(b) {
		return false
	}

	for key := range a {
		if _, found := b[key]; !found {
			return false
		}
	}

	return true
}
