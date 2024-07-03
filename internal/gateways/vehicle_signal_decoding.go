package gateways

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"

	"github.com/DIMO-Network/edge-network/config"

	"github.com/DIMO-Network/edge-network/internal/models"

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
	UpdateDeviceConfigStatus(ethAddr *common.Address, fwVersion string, unitID uuid.UUID, templateUrls *models.TemplateURLs) error
}

type vehicleSignalDecodingAPIService struct {
	httpClient shared.HTTPClientWrapper
	apiURL     string
}

// Environment define the environment type
type Environment int

const (
	Development Environment = iota
	Production
)

func (e Environment) String() string {
	return [...]string{"development", "prod"}[e]
}

func NewVehicleSignalDecodingAPIService(conf config.Config) VehicleSignalDecoding {
	h := map[string]string{}
	hcw, _ := shared.NewHTTPClientWrapper("", "", 10*time.Second, h, true) // ok to ignore err since only used for tor check

	return &vehicleSignalDecodingAPIService{
		httpClient: hcw,
		apiURL:     conf.Services.Vehicle.Host,
	}
}

func (v *vehicleSignalDecodingAPIService) GetPIDs(url string) (*models.TemplatePIDs, error) {
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
	res, err := v.httpClient.ExecuteRequest(fmt.Sprintf("%s/v1/device-config/vin/%s/urls", v.apiURL, vin), "GET", nil)
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
	res, err := v.httpClient.ExecuteRequest(fmt.Sprintf("%s/v1/device-config/eth-addr/%s/urls", v.apiURL, ethAddr), "GET", nil)
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
		return nil, errors.Wrapf(err, "error deserializing device settings from url %s. body: %s", url, string(bodyBytes))
	}

	return response, nil
}

func (v *vehicleSignalDecodingAPIService) GetDBC(url string) (*string, error) {
	h := map[string]string{}
	hcw, _ := shared.NewHTTPClientWrapper("", "", 10*time.Second, h, false)

	res, err := hcw.ExecuteRequest(url, "GET", nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error calling vehicle signal decoding api to get dbc from url %s", url)
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
		return nil, errors.Wrapf(err, "failed to get dbc from url %s", url)
	}
	resp := string(bodyBytes)

	return &resp, nil
}

func (v *vehicleSignalDecodingAPIService) UpdateDeviceConfigStatus(ethAddr *common.Address, fwVersion string, unitID uuid.UUID, templateUrls *models.TemplateURLs) error {
	// Construct the body using TemplateURLs
	body := &models.UpdateDeviceConfig{
		TemplateURLs:           *templateUrls,
		FirmwareVersionApplied: fwVersion,
	}

	// Convert the body to JSON
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return errors.Wrap(err, "failed to marshal body into JSON")
	}

	// sign  payload and add to headers
	hash := crypto.Keccak256(jsonBody)
	sig, err := commands.SignHash(unitID, hash)
	if err != nil {
		return errors.Wrap(err, "failed to sign the UpdateDeviceConfig")
	}
	signature := "0x" + hex.EncodeToString(sig)

	// Create headers for the request
	headers := map[string]string{
		"Signature": signature,
	}

	hcw, _ := shared.NewHTTPClientWrapper("", "", 10*time.Second, headers, true)
	url := fmt.Sprintf("%s/v1/device-config/eth-addr/%s/status", v.apiURL, ethAddr.String())
	res, err := hcw.ExecuteRequest(url, "PATCH", jsonBody)

	if err != nil {
		return errors.Wrapf(err, "error calling vehicle signal decoding api to update device config settings at url %s", url)
	}

	// Close the response body when the function returns
	defer res.Body.Close() // nolint

	// Handle the response status codes
	if res.StatusCode == 404 {
		return ErrNotFound
	}

	if res.StatusCode == 400 {
		return ErrBadRequest
	}

	return nil
}
