package commands

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
)

func DetectCanbus(unitID uuid.UUID) (canbusInfo CanbusInfo, err error) {
	req := executeRawRequest{Command: detectCanbusCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp obdAutoDetectResponse

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	canbusInfo = resp.CanbusInfo
	return
}

func GetVIN(unitID uuid.UUID) (vin string, err error) {

	req := executeRawRequest{Command: getVINCommand}
	url := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", url, req, &resp)
	if err != nil {
		return
	}

	vin = resp.Value
	if len(vin) != 17 {
		err = fmt.Errorf("response contained a VIN with %s characters", vin)
	}

	return
}

func ClearDiagnosticCodes(unitID uuid.UUID) (err error) {
	req := executeRawRequest{Command: clearDiagnosticCodeCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)

	if err != nil {
		return err
	}
	return
}

func GetDiagnosticCodes(unitID uuid.UUID) (codes string, err error) {
	req := executeRawRequest{Command: getDiagnosticCodeCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp dtcResponse
	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	log.Print("Response", resp)
	formattedResponse := ""
	for _, s := range resp.Values {
		formattedResponse += s.Code + ","
	}
	codes = strings.TrimSuffix(formattedResponse, ",")
	return
}
