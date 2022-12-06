package commands

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
)

func SignHash(unitID uuid.UUID, hash []byte) (sig []byte, err error) {
	hashHex := hex.EncodeToString(hash)

	req := executeRawRequest{Command: signHashCommand + hashHex}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	sig = common.FromHex(resp.Value)
	return
}

func GetEthereumAddress(unitID uuid.UUID) (addr common.Address, err error) {
	req := executeRawRequest{Command: getEthereumAddressCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp executeRawResponse

	err = executeRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	addr = common.HexToAddress(resp.Value)
	return
}
