package gateways

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/shared"
	"github.com/pkg/errors"
	"io"
	"time"
)

//go:generate mockgen -source identity_api.go -destination mocks/identity_api_mock.go
type IdentityApi interface {
	QueryIdentityAPIForVehicles(ethAddress string) ([]models.VehicleDefinition, error)
}

type identityAPIService struct {
	httpClient shared.HTTPClientWrapper
}

const IdentityAPIURL = "https://identity-api.dimo.zone/query"

func NewIdentityAPIService() IdentityApi {
	h := map[string]string{}
	h["Content-Type"] = "application/json"
	hcw, _ := shared.NewHTTPClientWrapper("", "", 10*time.Second, h, false) // ok to ignore err since only used for tor check

	return &identityAPIService{
		httpClient: hcw,
	}
}

func (i *identityAPIService) QueryIdentityAPIForVehicles(ethAddress string) ([]models.VehicleDefinition, error) {
	// GraphQL query
	graphqlQuery := `{
        vehicles(first: 10, filterBy: { owner: "` + ethAddress + `" }) {
            edges {
				node {
					tokenId,
					definition {
						make,
						model,
						year
					}
            	}
			}
        }
    }`

	return i.fetchVehiclesWithQuery(graphqlQuery)
}

func (i *identityAPIService) fetchVehiclesWithQuery(query string) ([]models.VehicleDefinition, error) {
	// GraphQL request
	requestPayload := models.GraphQLRequest{Query: query}
	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	// POST request
	res, err := i.httpClient.ExecuteRequest(IdentityAPIURL, "POST", payloadBytes)
	if err != nil {
		fmt.Print(err)
		if _, ok := err.(shared.HTTPResponseError); !ok {
			return nil, errors.Wrapf(err, "error calling identity api to get vehicles definition from url %s", IdentityAPIURL)
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
		return nil, errors.Wrapf(err, "error get vehicles definition from url %s", IdentityAPIURL)
	}

	var vehicleResponse struct {
		Data struct {
			Vehicles struct {
				Edges []struct {
					Node models.VehicleDefinition `json:"node"`
				} `json:"edges"`
			} `json:"vehicles"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &vehicleResponse); err != nil {
		return nil, err
	}

	vehicles := make([]models.VehicleDefinition, 0, len(vehicleResponse.Data.Vehicles.Edges))
	for _, edge := range vehicleResponse.Data.Vehicles.Edges {
		vehicles = append(vehicles, edge.Node)
	}

	return vehicles, nil
}
