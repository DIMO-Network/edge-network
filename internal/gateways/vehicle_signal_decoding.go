package gateways

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/models"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/pkg/errors"

	"github.com/DIMO-Network/shared"
)

var ErrNotFound = errors.New("not found")
var ErrBadRequest = errors.New("bad request")

//go:generate mockgen -source vehicle_signal_decoding.go -destination mocks/vehicle_signal_decoding_mock.go
type VehicleSignalDecoding interface {
	GetPIDs(url string) (*models.TemplatePIDs, error)
	GetUrlsByVin(vin string) (*models.TemplateURLs, error)
	GetUrlsByEthAddr(ethAddr *common.Address) (*models.TemplateURLs, error)
	GetDeviceSettings(url string) (*models.TemplateDeviceSettings, error)
	GetDBC(url string) (*string, error)
}

type vehicleSignalDecodingAPIService struct {
	httpClient shared.HTTPClientWrapper
}

const VehicleSignalDecodingAPIURL = "https://vehicle-signal-decoding.dimo.zone"

func NewVehicleSignalDecodingAPIService() VehicleSignalDecoding {
	h := map[string]string{}
	hcw, _ := shared.NewHTTPClientWrapper("", "", 10*time.Second, h, true) // ok to ignore err since only used for tor check

	return &vehicleSignalDecodingAPIService{
		httpClient: hcw,
	}
}

func (v *vehicleSignalDecodingAPIService) GetPIDs(url string) (*models.TemplatePIDs, error) {
	// todo: add retry
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

	response := new(models.TemplatePIDs)
	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return nil, errors.Wrapf(err, "error deserializing PID configurations from url %s", url)
	}

	return response, nil
}

// todo add method to get DBC's and device settings

func (v *vehicleSignalDecodingAPIService) GetUrlsByVin(vin string) (*models.TemplateURLs, error) {
	// todo: add retry
	res, err := v.httpClient.ExecuteRequest(fmt.Sprintf("%s/v1/device-config/vin/%s/urls", VehicleSignalDecodingAPIURL, vin), "GET", nil)
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

	response := new(models.TemplateURLs)
	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return nil, errors.Wrapf(err, "error deserializing URL configurations by vin %s", vin)
	}

	return response, nil
}

func (v *vehicleSignalDecodingAPIService) GetUrlsByEthAddr(ethAddr *common.Address) (*models.TemplateURLs, error) {
	// todo: add retry
	res, err := v.httpClient.ExecuteRequest(fmt.Sprintf("%s/v1/device-config/eth-addr/%s/urls", VehicleSignalDecodingAPIURL, ethAddr), "GET", nil)
	if err != nil {
		if _, ok := err.(shared.HTTPResponseError); !ok {
			return nil, errors.Wrapf(err, "error calling vehicle signal decoding api to get PID configurations by eth addr %s", ethAddr)
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
		return nil, errors.Wrapf(err, "error get URL configurations by eth addr %s", ethAddr)
	}

	response := new(models.TemplateURLs)
	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return nil, errors.Wrapf(err, "error deserializing URL configurations by eth addr %s", ethAddr)
	}

	return response, nil
}

func (v *vehicleSignalDecodingAPIService) GetDeviceSettings(url string) (*models.TemplateDeviceSettings, error) {
	// todo: add retry
	res, err := v.httpClient.ExecuteRequest(url, "GET", nil)
	if err != nil {
		if _, ok := err.(shared.HTTPResponseError); !ok {
			return nil, errors.Wrapf(err, "error calling vehicle signal decoding api to get device settings from url %s", url)
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
		return nil, errors.Wrapf(err, "error get device settings from url %s", url)
	}

	response := new(models.TemplateDeviceSettings)
	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return nil, errors.Wrapf(err, "error deserializing device settings from url %s", url)
	}

	return response, nil
}

func (v *vehicleSignalDecodingAPIService) GetDBC(url string) (*string, error) {
	res, err := v.httpClient.ExecuteRequest(url, "GET", nil)
	if err != nil {
		if _, ok := err.(shared.HTTPResponseError); !ok {
			return nil, errors.Wrapf(err, "error calling vehicle signal decoding api to get dbc from url %s", url)
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
		return nil, errors.Wrapf(err, "error get dbc from url %s", url)
	}
	resp := string(bodyBytes)

	return &resp, nil
}
