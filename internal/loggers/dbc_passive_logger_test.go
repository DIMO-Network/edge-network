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

//go:embed test_gm_tires_oil.dbc
var testgmmultipledbc string

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
			name:    "gm odo tires oil",
			dbcFile: testgmmultipledbc,
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
				{
					header: 1322,
					signals: []dbcSignal{
						{
							formula:    `16|8@1+ (4,0) [0|255] "kpa"`,
							signalName: "tiresFrontLeft",
						},
						{
							formula:    `24|8@1+ (4,0) [0|255] "kpa"`,
							signalName: "tiresBackLeft",
						},
						{
							formula:    `32|8@1+ (4,0) [0|255] "kpa"`,
							signalName: "tiresFrontRight",
						},
						{
							formula:    `40|8@1+ (4,0) [0|255] "kpa"`,
							signalName: "tiresBackRight",
						},
					},
				},
				{
					header: 1017,
					signals: []dbcSignal{
						{
							formula:    `48|8@1+ (0.392157,0) [0|255] "%"`,
							signalName: "oilLife",
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
	pidsObd2 := []models.PIDRequest{
		{
			Formula:         "dbc: 31|8@0+ (1,-40) [-40|215]",
			Header:          0x7df, // 7df
			IntervalSeconds: 60,
			Mode:            1, // 01
			Name:            "coolantTemp",
			Pid:             0x05, // 05
			Protocol:        "CAN11_500",
		},
		{
			Formula:         "dbc: 31|8@0+ (1,-40) [-40|215]",
			Header:          0x18db33f1, // default 29b
			IntervalSeconds: 60,
			Mode:            1, // 01
			Name:            "coolantTemp",
			Pid:             0x05, // 05
			Protocol:        "CAN29_500",
		},
	}
	pidsUds := []models.PIDRequest{
		{
			Formula:  "dbc: 31|8@0+ (1.8,-40) [-40|60]",
			Header:   0x7e6,
			Name:     "ambienAirTemp",
			Pid:      0xdd05,
			Protocol: "CAN11_500",
		},
		{
			Formula:  "dbc: 31|8@0+ (0.5,0) [0|1]",
			Header:   0x7e4,
			Name:     "soh",
			Pid:      0x490c,
			Protocol: "CAN11_500",
		},
	}

	tests := []struct {
		name        string
		frame       canbus.Frame
		wantPIDName string
		pids        []models.PIDRequest
	}{
		{
			name: "uds match ambient air temp",
			frame: canbus.Frame{
				ID:   0x7ee,
				Data: hexToByteArray("03 41 dd 05 34", t),
			},
			pids:        pidsUds,
			wantPIDName: "ambienAirTemp",
		},
		{
			name: "uds match soh",
			frame: canbus.Frame{
				ID:   0x7ec,
				Data: hexToByteArray("03 41 49 0c 37", t),
			},
			pids:        pidsUds,
			wantPIDName: "soh",
		},
		{
			name: "match coolant temp",
			frame: canbus.Frame{
				ID:   0x7e8,
				Data: hexToByteArray("03 41 05 53", t),
			},
			wantPIDName: "coolantTemp",
			pids:        pidsObd2,
		},
		{
			name: "match coolant temp EFF",
			frame: canbus.Frame{
				ID:   0x18daf133,
				Data: hexToByteArray("03 41 05 53", t),
			},
			wantPIDName: "coolantTemp",
			pids:        pidsObd2,
		},
		{
			name: "no match unregistered pid",
			frame: canbus.Frame{
				ID:   0x7e8,
				Data: hexToByteArray("10 14 49 02 01 4C 46 56", t),
			},
			wantPIDName: "",
			pids:        pidsObd2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dpl := &dbcPassiveLogger{
				logger:          testLogger,
				dbcFile:         nil,
				hardwareSupport: true,
				pids:            tt.pids,
			}
			match := dpl.matchPID(tt.frame)
			fmt.Printf("%+v\n", match)
			if tt.wantPIDName != "" {
				require.NotNil(t, match, "expected a match")
				assert.Equalf(t, tt.wantPIDName, match.Name, "matchPID(%v)", tt.frame)
			} else {
				assert.Nil(t, match, "expected nil match")
			}
		})
	}
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
					Header:               0x7df,
					CanFlowControlIDPair: "7df,123",
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
					Header:               0x7df,
					CanFlowControlIDPair: "7df,123",
				},
				{
					Header:               0x7df,
					CanFlowControlIDPair: "7df,456",
				},
				{
					Header:               0x7df,
					CanFlowControlIDPair: "7df,789",
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
					Header:               0x7df,
					CanFlowControlIDPair: "7df,123",
				},
				{
					Header:               0x7df,
					CanFlowControlIDPair: "7df,123",
				},
				{
					Header:               0x7df,
					CanFlowControlIDPair: "7df,123",
				},
			},
			want: map[uint32]struct{}{
				0x123: {},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := getUniqueResponseHeaders(tc.pids); !compareMaps(got, tc.want) {
				t.Errorf("getUniqueResponseHeaders() = %v, want %v", got, tc.want)
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

func Test_dbcPassiveLogger_ShouldNativeScanLogger(t *testing.T) {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	b := true
	type fields struct {
		hardwareSupport        bool
		pids                   []models.PIDRequest
		shouldNativeScanLogger *bool
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "no hw support",
			fields: fields{
				hardwareSupport: false,
				pids:            []models.PIDRequest{},
			},
			want: false,
		},
		{
			name: "all good",
			fields: fields{
				hardwareSupport: true,
				pids: []models.PIDRequest{
					{
						Formula: "dbc: 31|8 ",
					},
				},
			},
			want: true,
		},
		{
			name: "cached yes",
			fields: fields{
				shouldNativeScanLogger: &b,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dpl := &dbcPassiveLogger{
				logger:                 logger,
				hardwareSupport:        tt.fields.hardwareSupport,
				pids:                   tt.fields.pids,
				shouldNativeScanLogger: tt.fields.shouldNativeScanLogger,
			}
			assert.Equalf(t, tt.want, dpl.ShouldNativeScanLogger(), "ShouldNativeScanLogger()")
		})
	}
}
