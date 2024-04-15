package commands

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/models"
	"regexp"
	"strconv"
	"strings"
	"time"

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
func RequestPIDRaw(unitID uuid.UUID, request models.PIDRequest) (obdResp ObdResponse, ts time.Time, err error) {
	name := request.Name
	protocol, errProtocol := strconv.Atoi(request.Protocol)
	if errProtocol != nil {
		protocol = 6
	}
	pidHex := uintToHexStr(request.Pid)
	headerHex := fmt.Sprintf("%X", request.Header)
	modeHex := uintToHexStr(request.Mode)

	if !isValidHex(headerHex) {
		err = fmt.Errorf("header invalid %s", headerHex)
	}
	if !isValidHex(modeHex) {
		err = fmt.Errorf("mode invalid %s", modeHex)
	}
	if !isValidHex(pidHex) {
		err = fmt.Errorf("pid invalid %s", pidHex)
	}
	if err != nil {
		return
	}

	cmd := fmt.Sprintf(`%s %s header='"%s"' mode='x%s' pid='x%s' protocol=%d force=true verify=false`,
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
	fmt.Printf("DBG requesting PID: %s \n", cmd)

	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}
	switch v := resp.Value.(type) {
	case string:
		if isValidHexes(v) {
			obdResp.IsHex = true
			obdResp.ValueHex = strings.Split(v, "\n")

			for i := range obdResp.ValueHex {
				// add validation here for float values
				if len(obdResp.ValueHex[i]) > 0 && !isValidHex(obdResp.ValueHex[i]) {
					err = fmt.Errorf("invalid return value: %s", obdResp.ValueHex[i])
					return
				}
			}
		} else {
			obdResp.IsHex = false
			obdResp.Value = v
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
	ts, err = time.Parse("2006-01-02T15:04:05.000000", resp.Timestamp)
	ts = ts.UTC() // just in case

	return
}

// isValidHex checks if the input string is a valid hexadecimal.
func isValidHex(s string) bool {
	// Regex to match a valid hexadecimal string.
	// It starts with an optional "0x" or "0X", followed by one or more hexadecimal characters (0-9, a-f, A-F).
	re := regexp.MustCompile(`^(0x|0X)?[0-9a-fA-F]+$`)
	return re.MatchString(s)
}

func isValidHexes(s string) bool {
	// Regex to match one or multiple valid hexadecimal strings separated by a newline character.
	// Each hexadecimal string starts with an optional "0x" or "0X", followed by one or more hexadecimal characters (0-9, a-f, A-F).
	re := regexp.MustCompile(`^(0x|0X)?[0-9a-fA-F]+(\n(0x|0X)?[0-9a-fA-F]+)*$`)
	return re.MatchString(s)
}

// uintToHexStr converts the uint32 into a 0 padded hex representation, always assuming must be even length.
func uintToHexStr(val uint32) string {
	hexStr := fmt.Sprintf("%X", val)
	if len(hexStr)%2 != 0 {
		return "0" + hexStr // Prepend a "0" if the length is odd
	}
	return hexStr
}

/*
{
    "_type": "vin",
    "_stamp": "2024-02-29T17:17:30.534861",
    "value": "7e810144902014b4c34\n7e8214d4d44534c394c\n7e82242303631333534"
}
*/
