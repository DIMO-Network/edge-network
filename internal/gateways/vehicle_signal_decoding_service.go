package gateways

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"time"

	"github.com/DIMO-Network/shared"
)

var ErrNotFound = errors.New("not found")
var ErrBadRequest = errors.New("bad request")

//go:generate mockgen -source vehicle_signal_decoding_service.go -destination mocks/vehicle_signal_decoding_service_mock.go
type VehicleSignalDecodingAPIService interface {
	GetPIDsTemplateByVIN(vin string) (*PIDConfigResponse, error)
}

type PIDConfigResponse struct {
	ID       int64  `json:"id"`
	Header   string `json:"header"`
	Mode     string `json:"mode"`
	Pid      string `json:"pid"`
	Formula  string `json:"formula"`
	Protocol string `json:"protocol"`
}

type vehicleSignalDecodingAPIService struct {
	httpClient shared.HTTPClientWrapper
}

const VEHICLE_SIGNAL_DECODING_API_URL = "https://vehicle-signal-decoding.dev.dimo.zone"

func NewVehicleSignalDecodingAPIService() VehicleSignalDecodingAPIService {
	h := map[string]string{}
	hcw, _ := shared.NewHTTPClientWrapper(VEHICLE_SIGNAL_DECODING_API_URL, "", 10*time.Second, h, true) // ok to ignore err since only used for tor check

	return &vehicleSignalDecodingAPIService{
		httpClient: hcw,
	}
}

func (v *vehicleSignalDecodingAPIService) GetPIDsTemplateByVIN(vin string) (*PIDConfigResponse, error) {
	res, err := v.httpClient.ExecuteRequest(fmt.Sprintf("/v1/device-config/%s/pids", vin), "GET", nil)
	if err != nil {
		if _, ok := err.(shared.HTTPResponseError); !ok {
			return nil, errors.Wrapf(err, "error calling vehicle signal decoding api to get PID configurations by vin %s", vin)
		}
	}
	defer res.Body.Close() // nolint
	if res.StatusCode == 404 {
		return nil, ErrNotFound
	}

	if res.StatusCode == 400 {
		return nil, ErrBadRequest
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "error get PID configurations by vin %s", vin)
	}

	var response PIDConfigResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, errors.Wrapf(err, "error deserializing PID configurations by vin %s", vin)
	}

	return &response, nil
}
