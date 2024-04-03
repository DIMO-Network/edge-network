package commands

import (
	"encoding/hex"
	"errors"
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

	value, ok := resp.Value.(string)
	if !ok {
		return nil, errors.New("SignHash: value is not a string")
	}

	sig = common.FromHex(value)
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

	value, ok := resp.Value.(string)
	if !ok {
		return nil, errors.New("GetEthereumAddress: value is not a string")
	}

	ha := common.HexToAddress(value)
	addr = &ha
	return
}
