package commands

import (
	"encoding/hex"
	"fmt"

	"github.com/DIMO-Network/edge-network/internal/api"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
)

func SignHash(unitID uuid.UUID, hash []byte) (sig []byte, err error) {
	hashHex := hex.EncodeToString(hash)

	req := api.ExecuteRawRequest{Command: api.SignHashCommand + hashHex}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ExecuteRawResponse

	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	sig = common.FromHex(resp.Value)
	return
}

func GetEthereumAddress(unitID uuid.UUID) (addr *common.Address, err error) {
	req := api.ExecuteRawRequest{Command: api.GetEthereumAddressCommand}
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)

	var resp api.ExecuteRawResponse

	err = api.ExecuteRequest("POST", path, req, &resp)
	if err != nil {
		return
	}

	ha := common.HexToAddress(resp.Value)
	addr = &ha
	return
}
