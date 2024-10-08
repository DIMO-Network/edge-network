package network

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	dimoConfig "github.com/DIMO-Network/edge-network/config"
	"github.com/DIMO-Network/edge-network/internal/models"
	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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

	// here we see signature is getting set as expected, otherwise would be empty
	payload, err := sjson.Set(payload, "signature", "0xb794f5ea0ba39494ce")
	require.NoError(t, err)
	mockClient.EXPECT().Publish("topic", uint8(1), false, []byte(payload)).Times(1).Return(&mockedToken{})

	path := fmt.Sprintf("/dongle/%s/execute_raw", ds.unitID.String())
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+path,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce"}`))

	err = ds.sendPayload("topic", []byte(payload), false)
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

	config, confErr := dimoConfig.ReadConfigFromPath("../../config-dev.yaml")
	if confErr != nil {
		testLogger.Fatal().Err(confErr).Msg("unable to read config file")
	}
	ds := &dataSender{
		client:  mockClient,
		unitID:  uuid.New(),
		ethAddr: common.HexToAddress("0x694C9A19e3644A9BFe1008857aeEd155F27b078e"),
		logger:  testLogger,
		mqtt:    config.Mqtt,
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

	status := ds.mqtt.Topics.Status
	// if the status topic has a %s in it, replace it with the subject
	// this is needed for backwards compatibility with the old topic format serving by mosquito
	if strings.Contains(status, "%s") {
		status = fmt.Sprintf(status, ds.ethAddr.Hex())
	}
	mockClient.EXPECT().Publish(status, uint8(1), false, gomock.Any()).Times(1).Return(&mockedToken{})

	path := fmt.Sprintf("/dongle/%s/execute_raw", ds.unitID.String())
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+path,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce"}`))

	err := ds.SendDeviceStatusData(deviceStatusData)
	require.NoError(t, err)
}

func Test_compressDeviceStatusData(t *testing.T) {
	// given
	deviceStatusData := models.DeviceStatusData{
		CommonData: models.CommonData{
			Timestamp: 0,
		},
		Device: models.Device{
			BatteryVoltage: 13.3,
			RpiUptimeSecs:  2,
		},
	}
	deviceStatusDataBytes, _ := json.Marshal(deviceStatusData)

	// when
	// then
	compressedData, _ := compressPayload(deviceStatusDataBytes)
	decoded, _ := base64.StdEncoding.DecodeString(compressedData.Payload)
	bytesArr, err := decompressGzip(decoded)
	if err != nil {
		t.Errorf("error decompressing gzip: %v", err)
	}

	// verify
	var data models.DeviceStatusData
	err = json.Unmarshal(bytesArr, &data)
	assert.Nil(t, err)
	assert.Equal(t, 13.3, data.Device.BatteryVoltage)
	assert.Equal(t, 2, data.Device.RpiUptimeSecs)
}

func decompressGzip(data []byte) ([]byte, error) {
	b := bytes.NewBuffer(data)
	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
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
