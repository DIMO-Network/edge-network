package internal

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	mock_loggers "github.com/DIMO-Network/edge-network/internal/loggers/mocks"
	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func Test_loggerService_StartLoggers(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"
	const vinDiesel = "5TFCZ5AN0HX073768"

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	const vinQueryName = "vin_7DF_09_02"
	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	unitID := uuid.New()
	loggerSettings := &LoggerSettings{VINQueryName: vinQueryName}
	ls := NewLoggerService(unitID, vl, ds, loggerSettings)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))
	// mock eth addr
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce839613fffba74279579268"}`))

	vl.EXPECT().GetVIN(unitID, nil).Times(1).Return(&loggers.VINResponse{VIN: vinDiesel, Protocol: "6", QueryName: vinQueryName}, nil)
	ds.EXPECT().SendPayload(gomock.Any(), unitID).Times(1).Return(nil)

	err := ls.StartLoggers()

	assert.NoError(t, err)
}
