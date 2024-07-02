package loggers

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/DIMO-Network/edge-network/internal/loggers/canbus"
	"github.com/DIMO-Network/edge-network/internal/models"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

//go:embed test_gm120.dbc
var testgm120dbc string

func Test_dbcPassiveLogger_parseDBCHeaders(t *testing.T) {
	testLogger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})

	tests := []struct {
		name    string
		dbcFile string
		want    []dbcFilter
	}{
		{
			name:    "gm odometer",
			dbcFile: testgm120dbc,
			want: []dbcFilter{
				{
					header:     288,
					formula:    `7|32@0+ (0.015625,0) [0|67108863.984375] "km" Vector_XXX`,
					signalName: "odometer",
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
			assert.NoError(t, err)
			assert.Equal(t, tt.want[0], parsed[0])
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
