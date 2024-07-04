package commands

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/util"
	"strconv"
	"strings"
	"time"

	"github.com/DIMO-Network/edge-network/internal/models"

	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/google/uuid"
)

type ObdResponse struct {
	IsHex bool
	// ValueHex is a slice of hex strings, each string is a response from the OBD device.
	ValueHex []string
	// Value is the response from the OBD device in string or float format.
	Value any
}

func DetectCanbus(unitID uuid.UUID) (canbusInfo api.CanbusInfo, err error) {
	req := api.ExecuteRawRequest{Command: api.DetectCanbusCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ObdAutoDetectResponse

	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	canbusInfo = resp.CanbusInfo
	return
}

func ClearDiagnosticCodes(unitID uuid.UUID) (err error) {
	req := api.ExecuteRawRequest{Command: api.ClearDiagnosticCodeCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ExecuteRawResponse

	err = api.ExecuteRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func GetDiagnosticCodes(unitID uuid.UUID, logger zerolog.Logger) (codes string, err error) {
	codes = ""
	req := api.ExecuteRawRequest{Command: api.GetDiagnosticCodeCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.DTCResponse
	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	logger.Info().Msgf("Response %s", resp)
	formattedResponse := ""
	for _, s := range resp.Values {
		formattedResponse += s.Code + ","
	}
	codes = strings.TrimSuffix(formattedResponse, ",")

	return
}

// RequestPIDRaw requests a pid via obd. Whatever calls this should be using a mutex to avoid calling while another in process, avoid overloading canbus
func RequestPIDRaw(logger *zerolog.Logger, unitID uuid.UUID, request models.PIDRequest) (obdResp ObdResponse, ts time.Time, err error) {
	name := request.Name
	protocol, errProtocol := strconv.Atoi(request.Protocol)
	if errProtocol != nil {
		protocol = 6
	}
	pidHex := util.UintToHexStr(request.Pid)
	headerHex := fmt.Sprintf("%X", request.Header)
	modeHex := util.UintToHexStr(request.Mode)

	if !util.IsValidHex(headerHex) {
		err = fmt.Errorf("header invalid %s", headerHex)
	}
	if !util.IsValidHex(modeHex) {
		err = fmt.Errorf("mode invalid %s", modeHex)
	}
	if !util.IsValidHex(pidHex) {
		err = fmt.Errorf("pid invalid %s", pidHex)
	}
	if err != nil {
		return
	}
	// verify=false optionally add here depending, maybe pass as a parameter
	cmd := fmt.Sprintf(`%s %s header='"%s"' mode='x%s' pid='x%s' protocol=%d force=true`,
		api.ObdPIDQueryCommand, name, headerHex, modeHex, pidHex, protocol)

	if request.FormulaType() == models.Python {
		cmd = fmt.Sprintf(`%s formula='%s'`, cmd, request.FormulaValue())
	}

	if request.CanflowControlClear {
		cmd = fmt.Sprintf(`%s flow_control_clear=true`, cmd)
	}

	if request.CanFlowControlIDPair != "" {
		cmd = fmt.Sprintf(`%s flow_control_id_pair='%s'`, cmd, request.CanFlowControlIDPair)
	}

	req := api.ExecuteRawRequest{Command: cmd}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ExecuteRawResponse
	logger.Debug().Msgf("requesting PID: %s", cmd)

	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}
	logger.Debug().Msgf("response for %s: %s", request.Name, resp.Value)

	switch v := resp.Value.(type) {
	case string:
		if request.FormulaType() == models.Python { // formula was set to python, autopi processed it
			if v == "" {
				err = fmt.Errorf("empty response with formula: %s", request.Formula)
				return
			}
			obdResp.IsHex = false
			obdResp.Value = v
		} else if util.IsHexFrames(v) {
			obdResp.IsHex = true
			// clean autopi multiframe start characters
			frames := strings.Split(v, "\n")
			if len(frames) > 0 && frames[0] == "|-" {
				frames = append(frames[:0], frames[1:]...)
			}
			obdResp.ValueHex = frames
		} else {
			err = fmt.Errorf("invalid return value: %s", v)
			return
		}
	case float64:
		// the int value always unmarshal to float, that's why we
		// only handle float64
		obdResp.IsHex = false
		obdResp.Value = v
	default:
		err = fmt.Errorf("invalid response type: %T", v)
	}
	if obdResp.IsHex && len(obdResp.ValueHex) == 0 {
		err = fmt.Errorf("no response received")
	}
	ts, errParse := time.Parse("2006-01-02T15:04:05.000000", resp.Timestamp)
	ts = ts.UTC() // just in case
	if errParse != nil {
		err = fmt.Errorf("error parsing timestamp: %w", errParse)
	}

	return
}

/*
{
    "_type": "vin",
    "_stamp": "2024-02-29T17:17:30.534861",
    "value": "7e810144902014b4c34\n7e8214d4d44534c394c\n7e82242303631333534"
}
*/
