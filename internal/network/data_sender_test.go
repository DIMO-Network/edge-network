package network

import (
	"fmt"
	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"net/http"
	"os"
	"testing"
	"time"
)

//go:generate mockgen -destination=mocks/mqtt_mock.go -package=mock_network github.com/eclipse/paho.mqtt.golang Client

func Test_dataSender_sendPayload(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	testLogger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})

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
	mockClient.EXPECT().Connect().Times(1).Return(mockedToken{})
	mockClient.EXPECT().IsConnected().Times(1).Return(true)
	mockClient.EXPECT().Disconnect(gomock.Any())
	mockClient.EXPECT().Publish("topic", 0, false, payload).Times(1).Return(mockedToken{})
	// todo register correct thing for signing
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", ds.unitID.String())
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce839613fffba74279579268"}`))

	err := ds.sendPayload("topic", []byte(payload))
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
