package commands

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"os"
	"testing"
)

func Test_isValidHex(t *testing.T) {
	tests := []struct {
		name     string
		hex      string
		want     bool
		starts0x bool
	}{
		{
			hex:      "0x1A3F",
			want:     true,
			starts0x: true,
		},
		{
			hex:      "0X4D52",
			want:     true,
			starts0x: true,
		},
		{
			hex:      "7DF",
			want:     true,
			starts0x: false,
		},
		{
			hex:  "88Z1",
			want: false,
		},
		{
			hex:  "0x",
			want: false,
		},
		{
			hex:  "7E8101462F190574155",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			if got := isValidHex(tt.hex); got != tt.want {
				t.Errorf("isValidHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isHexFrames(t *testing.T) {
	tests := []struct {
		name         string
		hexMultiLine string
		want         bool
	}{
		{
			name:         "vin multi frame",
			hexMultiLine: "|-\n7E8101462F190574155\n7E8215247423852324C\n7E8224E303036323232",
			want:         true,
		},
		{
			name:         "single line",
			hexMultiLine: "7DF",
			want:         true,
		},
		{
			name:         "invalid",
			hexMultiLine: "88Z1",
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, isHexFrames(tt.hexMultiLine), "isHexFrames(%v)", tt.hexMultiLine)
		})
	}
}

func TestRequestPIDRaw_table(t *testing.T) {
	logger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})
	// formula only used if type python
	testCases := []struct {
		name                string
		inputPIDRequest     models.PIDRequest
		obdQuery            string
		respBody            string
		expectedValueHex    []string
		expectedValueParsed any
		expectedError       string // if not empty error contains this
	}{
		{
			"Happy path hex and dbc",
			models.PIDRequest{
				Name:            "fuellevel",
				IntervalSeconds: 60,
				Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
			},
			"obd.query fuellevel header='\"0\"' mode='x00' pid='x00' protocol=6 force=true",
			`{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`,
			[]string{"7e803412f6700000000"},
			nil,
			"",
		},
		{
			"invalid hex response",
			models.PIDRequest{
				Name:            "fuellevel",
				IntervalSeconds: 60,
			},
			"obd.query fuellevel header='\"0\"' mode='x00' pid='x00' protocol=6 force=true",
			`{"value": "SEARCHING...", "_stamp": "2024-02-29T17:17:30.534861"}`,
			nil,
			nil,
			"invalid return value",
		},
		{
			"invalid python autopi formula response",
			models.PIDRequest{
				Name:            "fuellevelfailure",
				IntervalSeconds: 60,
				Formula:         "python: xxx",
			},
			"obd.query fuellevelfailure header='\"0\"' mode='x00' pid='x00' protocol=6 force=true formula=' xxx'",
			`{"value": "", "_stamp": "2024-02-29T17:17:30.534861"}`,
			nil,
			nil,
			"empty response",
		},
		{
			"query with can flow control",
			models.PIDRequest{
				Name:                 "fuellevel",
				IntervalSeconds:      60,
				Formula:              "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
				CanflowControlClear:  true,
				CanFlowControlIDPair: "744,7AE",
			},
			"obd.query fuellevel header='\"0\"' mode='x00' pid='x00' protocol=6 force=true flow_control_clear=true flow_control_id_pair='744,7AE'",
			`{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`,
			[]string{"7e803412f6700000000"},
			nil,
			"",
		},
		{
			"python formula type with float resp",
			models.PIDRequest{
				Name:            "fuellevel",
				IntervalSeconds: 60,
				Formula:         "python:bytes_to_int(messages[0].data[-2:])*0.1",
			},
			"obd.query fuellevel header='\"0\"' mode='x00' pid='x00' protocol=6 force=true formula='bytes_to_int(messages[0].data[-2:])*0.1'",
			`{"value": 2.3, "_stamp": "2024-02-29T17:17:30.534861"}`,
			[]string{},
			2.3,
			"",
		},
		{
			"python formula type with string resp",
			models.PIDRequest{
				Name:            "gearselected",
				IntervalSeconds: 60,
				Formula:         "python:bytes_to_int(messages[0].data[-2:])*0.1",
			},
			"obd.query gearselected header='\"0\"' mode='x00' pid='x00' protocol=6 force=true formula='bytes_to_int(messages[0].data[-2:])*0.1'",
			`{"value": "D", "_stamp": "2024-02-29T17:17:30.534861"}`,
			[]string{},
			"D",
			"",
		},
		// Add more test cases here
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			unitID := uuid.New()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			psPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
			registerResponderAndAssert(t, psPath, tc.obdQuery,
				tc.respBody)
			obdResp, _, err := RequestPIDRaw(&logger, unitID, tc.inputPIDRequest)

			if tc.expectedError != "" {
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.Nil(t, err)
			}

			if len(tc.expectedValueHex) > 0 {
				assert.Equal(t, true, obdResp.IsHex)
				assert.NotNil(t, obdResp.ValueHex)
				assert.Equal(t, tc.expectedValueHex, obdResp.ValueHex)
			}
			if tc.expectedValueParsed != nil {
				assert.False(t, obdResp.IsHex)
				assert.Nil(t, obdResp.ValueHex)
				assert.Equal(t, tc.expectedValueParsed, obdResp.Value)
			}
		})
	}

}

func registerResponderAndAssert(t *testing.T, psPath string, cmd string, body string) {
	httpmock.RegisterResponderWithQuery(http.MethodPost, psPath, nil,
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), err
			}

			var request api.ExecuteRawRequest
			err = json.Unmarshal(bodyBytes, &request)
			if err != nil {
				assert.Error(t, err)
			}

			// check if the request body contains the expected bytes
			assert.Equal(t, cmd, request.Command)

			return httpmock.NewStringResponse(200, body), nil
		})
}
