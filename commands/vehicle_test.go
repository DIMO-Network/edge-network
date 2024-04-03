package commands

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"io/ioutil"
	"net/http"
	"testing"
)

func Test_isValidHex(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want bool
	}{
		{
			hex:  "0x1A3F",
			want: true,
		},
		{
			hex:  "0X4D52",
			want: true,
		},
		{
			hex:  "7DF",
			want: true,
		},
		{
			hex:  "88Z1",
			want: false,
		},
		{
			hex:  "0x",
			want: false,
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

func TestRequestPID(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// mock pid resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	registerResponderAndAssert(t, psPath, "obd.query fuellevel header=\"'0'\" mode='x00' pid='x00' protocol=6 force=true",
		`{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`)

	request := models.PIDRequest{
		Name:            "fuellevel",
		IntervalSeconds: 60,
		Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
	}

	// then
	obdResp, _, err := RequestPIDRaw(unitID, request)

	// verify
	assert.Nil(t, err)
	assert.True(t, obdResp.IsHex)
	assert.NotNil(t, obdResp.ValueHex)
	assert.Equal(t, 1, len(obdResp.ValueHex))
}

func TestRequestPIDWithCanFlowControl(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// mock pid resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	registerResponderAndAssert(t, psPath, "obd.query fuellevel header=\"'0'\" mode='x00' pid='x00' protocol=6 force=true flow_control_clear=true flow_control_id_pair='744,7AE'",
		`{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`)

	request := models.PIDRequest{
		Name:                 "fuellevel",
		IntervalSeconds:      60,
		Formula:              "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		CanflowControlClear:  true,
		CanFlowControlIDPair: "744,7AE",
	}

	// then
	obdResp, _, err := RequestPIDRaw(unitID, request)

	// verify
	assert.Nil(t, err)
	assert.True(t, obdResp.IsHex)
	assert.NotNil(t, obdResp.ValueHex)
	assert.Equal(t, 1, len(obdResp.ValueHex))
}

func TestRequestPIDFormulaTypePython(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// mock pid resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	registerResponderAndAssert(t, psPath, "obd.query fuellevel header=\"'0'\" mode='x00' pid='x00' protocol=6 force=true formula='bytes_to_int(messages[0].data[-2:])*0.1'",
		`{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`)

	request := models.PIDRequest{
		Name:            "fuellevel",
		IntervalSeconds: 60,
		Formula:         "python:bytes_to_int(messages[0].data[-2:])*0.1",
	}

	// then
	obdResp, _, err := RequestPIDRaw(unitID, request)

	// verify
	assert.Nil(t, err)
	assert.True(t, obdResp.IsHex)
	assert.NotNil(t, obdResp.ValueHex)
	assert.Equal(t, 1, len(obdResp.ValueHex))
}

func TestRequestPIDFormulaTypePythonWithMultipleHex(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// mock pid resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	registerResponderAndAssert(t, psPath, "obd.query foo header=\"'0'\" mode='x00' pid='x00' protocol=6 force=true formula='bytes_to_int(messages[0].data[-2:])*0.1'",
		`{"value": "7e803412f6700000000\n7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`)

	request := models.PIDRequest{
		Name:            "foo",
		IntervalSeconds: 60,
		Formula:         "python:bytes_to_int(messages[0].data[-2:])*0.1",
	}

	// then
	obdResp, _, err := RequestPIDRaw(unitID, request)

	// verify
	assert.Nil(t, err)
	assert.True(t, obdResp.IsHex)
	assert.NotNil(t, obdResp.ValueHex)
	assert.Equal(t, 2, len(obdResp.ValueHex))
}

func TestRequestPIDFormulaTypePythonWithFloatValue(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// mock pid resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	registerResponderAndAssert(t, psPath, "obd.query airtemp header=\"'0'\" mode='x00' pid='x00' protocol=6 force=true formula='bytes_to_int(messages[0].data[-2:]) * 0.001'",
		`{"value": 17.92, "_stamp": "2024-02-29T17:17:30.534861"}`)

	request := models.PIDRequest{
		Name:            "airtemp",
		IntervalSeconds: 60,
		Formula:         "python:bytes_to_int(messages[0].data[-2:]) * 0.001",
	}

	// then
	obdResp, _, err := RequestPIDRaw(unitID, request)

	// verify
	assert.Nil(t, err)
	assert.NotNil(t, obdResp)
	assert.True(t, !obdResp.IsHex)
	assert.Equal(t, 17.92, obdResp.Value)
}

func TestRequestPIDFormulaTypePythonWithStringValue(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// mock pid resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	registerResponderAndAssert(t, psPath, "obd.query airtemp header=\"'0'\" mode='x00' pid='x00' protocol=6 force=true formula='bytes_to_int(messages[0].data[-2:]) * 0.001'",
		`{"value": "17.92", "_stamp": "2024-02-29T17:17:30.534861"}`)

	request := models.PIDRequest{
		Name:            "airtemp",
		IntervalSeconds: 60,
		Formula:         "python:bytes_to_int(messages[0].data[-2:]) * 0.001",
	}

	// then
	obdResp, _, err := RequestPIDRaw(unitID, request)

	// verify
	assert.Nil(t, err)
	assert.NotNil(t, obdResp)
	assert.True(t, !obdResp.IsHex)
	assert.Equal(t, "17.92", obdResp.Value)
}

func registerResponderAndAssert(t *testing.T, psPath string, cmd string, body string) {
	httpmock.RegisterResponderWithQuery(http.MethodPost, psPath, nil,
		func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := ioutil.ReadAll(req.Body)
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
