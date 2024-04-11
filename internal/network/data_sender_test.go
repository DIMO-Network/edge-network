package network

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -destination=mocks/mqtt_mock.go -package=mock_network github.com/eclipse/paho.mqtt.golang Client

func Test_dataSender_sendPayload(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	testLogger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	mockClient := mock_network.NewMockClient(mockCtrl)
	ds := &dataSender{
		client:  mockClient,
		unitID:  uuid.New(),
		ethAddr: common.HexToAddress("0x694C9A19e3644A9BFe1008857aeEd155F27b078e"),
		logger:  testLogger,
	}
	payload := `{"subject": "%s", "signature": "", "source":"aftermarket/device/status", "data": {"rpiUptimeSecs":200,"batteryVoltage":13.6,"timestamp":1709140771210 } }`
	payload = fmt.Sprintf(payload, ds.ethAddr.Hex())

	// expectations
	mockClient.EXPECT().Connect().Times(1).Return(&mockedToken{})
	mockClient.EXPECT().IsConnected().Times(1).Return(true)
	mockClient.EXPECT().Disconnect(gomock.Any())
	// here we see signature is getting set as expected, otherwise would be empty
	payload, err := sjson.Set(payload, "signature", "0xb794f5ea0ba39494ce")
	require.NoError(t, err)
	mockClient.EXPECT().Publish("topic", uint8(0), false, payload).Times(1).Return(&mockedToken{})

	path := fmt.Sprintf("/dongle/%s/execute_raw", ds.unitID.String())
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+path,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce"}`))

	err = ds.sendPayload("topic", []byte(payload))
	require.NoError(t, err)

}

func Test_dataSender_sendPayloadWithVehicleInfo(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	testLogger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	mockClient := mock_network.NewMockClient(mockCtrl)
	ds := &dataSender{
		client:  mockClient,
		unitID:  uuid.New(),
		ethAddr: common.HexToAddress("0x694C9A19e3644A9BFe1008857aeEd155F27b078e"),
		logger:  testLogger,
		vehicleInfo: &models.VehicleInfo{
			TokenID: 12345,
			VehicleDefinition: models.VehicleDefinition{
				Make:  "Toyota",
				Model: "Corolla",
				Year:  2022,
			},
		},
	}
	deviceStatusData := models.DeviceStatusData{
		CommonData: models.CommonData{
			Timestamp: time.Now().UTC().UnixMilli(),
		},
		Device: models.Device{
			RpiUptimeSecs:  200,
			BatteryVoltage: 13.6,
		},
		Vehicle: models.Vehicle{
			Signals: []models.SignalData{
				{
					Timestamp: time.Now().UTC().UnixMilli(),
					Name:      "Speed",
					Value:     60,
				},
			},
		},
	}

	// expectations
	mockClient.EXPECT().Connect().Times(1).Return(&mockedToken{})
	mockClient.EXPECT().IsConnected().Times(1).Return(true)
	mockClient.EXPECT().Disconnect(gomock.Any())
	mockClient.EXPECT().Publish("status", uint8(0), false, gomock.Any()).Times(1).
		Do(func(_ string, qos uint8, retained bool, payload string) {
			assert.True(t, strings.Contains(payload, "tokenId"), "Payload does not contain tokenID")
		}).Return(&mockedToken{})

	path := fmt.Sprintf("/dongle/%s/execute_raw", ds.unitID.String())
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+path,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce"}`))

	err := ds.SendDeviceStatusData(deviceStatusData)
	require.NoError(t, err)
}

type mockedToken struct {
}

func (t *mockedToken) Wait() bool {
	return false
}
func (t *mockedToken) WaitTimeout(time.Duration) bool {
	return false
}
func (t *mockedToken) Done() <-chan struct{} {
	return nil
}
func (t *mockedToken) Error() error {
	return nil
}
