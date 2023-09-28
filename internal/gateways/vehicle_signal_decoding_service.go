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
	GetPIDs(url string) (*PIDConfigResponse, error)
	GetUrls(vin string) (*UrlConfigResponse, error)
}

type UrlConfigResponse struct {
	PidURL           string `json:"pidUrl"`
	DeviceSettingURL string `json:"deviceSettingUrl"`
	DbcURL           string `json:"dbcURL"`
	Version          string `json:"version"`
}

type PIDConfigResponse struct {
	Requests     []PIDConfigItemResponse `json:"requests"`
	TemplateName string                  `json:"template_name"`
	Version      string                  `json:"version"`
}

type PIDConfigItemResponse struct {
	ID              int64  `json:"id"`
	Header          int64  `json:"header"`
	Mode            int64  `json:"mode"`
	Pid             int64  `json:"pid"`
	Formula         string `json:"formula"`
	Protocol        string `json:"protocol"`
	IntervalSeconds int    `json:"interval_seconds"`
	Name            int64  `json:"name"`
	Version         string `json:"version"`
}

type vehicleSignalDecodingAPIService struct {
	httpClient shared.HTTPClientWrapper
}

const VehicleSignalDecodingApiUrl = "https://vehicle-signal-decoding.dimo.zone"

func NewVehicleSignalDecodingAPIService() VehicleSignalDecodingAPIService {
	h := map[string]string{}
	hcw, _ := shared.NewHTTPClientWrapper("", "", 10*time.Second, h, true) // ok to ignore err since only used for tor check

	return &vehicleSignalDecodingAPIService{
		httpClient: hcw,
	}
}

func (v *vehicleSignalDecodingAPIService) GetPIDs(url string) (*PIDConfigResponse, error) {
	res, err := v.httpClient.ExecuteRequest(url, "GET", nil)
	if err != nil {
		if _, ok := err.(shared.HTTPResponseError); !ok {
			return nil, errors.Wrapf(err, "error calling vehicle signal decoding api to get PID configurations from url %s", url)
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
		return nil, errors.Wrapf(err, "error get PID configurations from url %s", url)
	}

	var response PIDConfigResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, errors.Wrapf(err, "error deserializing PID configurations from url %s", url)
	}

	return &response, nil
}

func (v *vehicleSignalDecodingAPIService) GetUrls(vin string) (*UrlConfigResponse, error) {
	res, err := v.httpClient.ExecuteRequest(fmt.Sprintf("%s/v1/device-config/vin/%s/urls", VehicleSignalDecodingApiUrl, vin), "GET", nil)
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
		return nil, errors.Wrapf(err, "error get URL configurations by vin %s", vin)
	}

	var response UrlConfigResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, errors.Wrapf(err, "error deserializing URL configurations by vin %s", vin)
	}

	return &response, nil
}
