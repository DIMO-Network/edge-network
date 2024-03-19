package commands

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/google/uuid"
)

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
func RequestPIDRaw(unitID uuid.UUID, name, headerHex, modeHex, pidHex string, protocol int) (hexResp []string, ts time.Time, err error) {
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

	cmd := fmt.Sprintf(`%s %s header="'%s'" mode='x%s' pid='x%s' protocol=%d force=true`,
		api.ObdPIDQueryCommand, name, headerHex, modeHex, pidHex, protocol)
	req := api.ExecuteRawRequest{Command: cmd}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ExecuteRawResponse
	fmt.Printf("DBG requesting PID: %s \n", cmd)

	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}
	hexResp = strings.Split(resp.Value, "\n")
	for i := range hexResp {
		if len(hexResp[i]) > 0 && !isValidHex(hexResp[i]) {
			err = fmt.Errorf("invalid return value: %s", hexResp[i])
			return
		}
	}
	if len(hexResp) == 0 {
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

/*
{
    "_type": "vin",
    "_stamp": "2024-02-29T17:17:30.534861",
    "value": "7e810144902014b4c34\n7e8214d4d44534c394c\n7e82242303631333534"
}
*/
