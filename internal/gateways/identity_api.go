package gateways

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/DIMO-Network/edge-network/config"

	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/shared"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

//go:generate mockgen -source identity_api.go -destination mocks/identity_api_mock.go
type IdentityAPI interface {
	QueryIdentityAPIForVehicle(ethAddress common.Address) (*models.VehicleInfo, error)
}

type identityAPIService struct {
	httpClient shared.HTTPClientWrapper
	logger     zerolog.Logger
	apiURL     string
}

func NewIdentityAPIService(logger zerolog.Logger, config config.Config) IdentityAPI {
	h := map[string]string{}
	h["Content-Type"] = "application/json"
	hcw, _ := shared.NewHTTPClientWrapper("", "", 10*time.Second, h, false) // ok to ignore err since only used for tor check

	return &identityAPIService{
		httpClient: hcw,
		logger:     logger,
		apiURL:     config.Services.Identity.Host,
	}
}

func (i *identityAPIService) QueryIdentityAPIForVehicle(ethAddress common.Address) (*models.VehicleInfo, error) {
	// GraphQL query
	graphqlQuery := `{
        aftermarketDevice(by: {address: "` + ethAddress.Hex() + `"}) {
			vehicle {
			  tokenId,
			  definition {
				make
				model
				year
			  }
			}
  		}
	}`

	return i.fetchVehicleWithQuery(graphqlQuery)
}

func (i *identityAPIService) fetchVehicleWithQuery(query string) (*models.VehicleInfo, error) {
	// GraphQL request
	requestPayload := models.GraphQLRequest{Query: query}
	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	// POST request
	res, err := i.httpClient.ExecuteRequest(i.apiURL, "POST", payloadBytes)
	if err != nil {
		i.logger.Err(err).Send()
		if _, ok := err.(shared.HTTPResponseError); !ok {
			return nil, errors.Wrapf(err, "error calling identity api to get vehicles definition from url %s", i.apiURL)
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
		return nil, errors.Wrapf(err, "error get vehicles definition from url %s", i.apiURL)
	}

	var vehicleResponse struct {
		Data struct {
			AfterMarketDevice struct {
				Vehicle models.VehicleInfo `json:"vehicle"`
			} `json:"aftermarketDevice"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &vehicleResponse); err != nil {
		return nil, err
	}

	if vehicleResponse.Data.AfterMarketDevice.Vehicle.TokenID == 0 {
		return nil, Stop{fmt.Errorf(ErrNotFound.Error())}
	}

	return &vehicleResponse.Data.AfterMarketDevice.Vehicle, nil
}
