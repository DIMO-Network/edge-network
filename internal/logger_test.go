package internal

import (
	"fmt"
	"github.com/rs/zerolog"
	"net/http"
	"os"
	"testing"

	mock_gateways "github.com/DIMO-Network/edge-network/internal/gateways/mocks"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	mock_loggers "github.com/DIMO-Network/edge-network/internal/loggers/mocks"
	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func Test_loggerService_VINLoggers(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"
	const vinDiesel = "5TFCZ5AN0HX073768"

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vinQueryName := "vin_7DF_09_02"
	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	unitID := uuid.New()
	lss := mock_loggers.NewMockLoggerSettingsService(mockCtrl)

	pidl := mock_loggers.NewMockPIDLogger(mockCtrl)
	vsd := mock_gateways.NewMockVehicleSignalDecodingAPIService(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	ls := NewLoggerService(unitID, vl, pidl, ds, lss, vsd, logger)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))
	// mock eth addr
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce839613fffba74279579268"}`))

	lss.EXPECT().ReadVINConfig().Times(1).Return(&loggers.VINLoggerSettings{VINQueryName: vinQueryName}, nil)
	vl.EXPECT().GetVIN(unitID, &vinQueryName).Times(1).Return(&loggers.VINResponse{VIN: vinDiesel, Protocol: "6", QueryName: vinQueryName}, nil)
	ds.EXPECT().SendFingerprintData(gomock.Any()).Times(1).Return(nil)

	err := ls.Fingerprint()

	assert.NoError(t, err)
}

func Test_loggerService_VINLoggers_nilSettings(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"
	const vinDiesel = "5TFCZ5AN0HX073768"

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vinQueryName := "vin_7DF_09_02"
	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	unitID := uuid.New()
	lss := mock_loggers.NewMockLoggerSettingsService(mockCtrl)

	pidl := mock_loggers.NewMockPIDLogger(mockCtrl)
	vsd := mock_gateways.NewMockVehicleSignalDecodingAPIService(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	ls := NewLoggerService(unitID, vl, pidl, ds, lss, vsd, logger)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))
	// mock eth addr
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce839613fffba74279579268"}`))

	lss.EXPECT().ReadVINConfig().Times(1).Return(nil, fmt.Errorf("error reading file: open /tmp/logger-settings.json: no such file or directory"))
	vl.EXPECT().GetVIN(unitID, nil).Times(1).Return(&loggers.VINResponse{VIN: vinDiesel, Protocol: "6", QueryName: vinQueryName}, nil)
	lss.EXPECT().WriteVINConfig(loggers.VINLoggerSettings{VINQueryName: vinQueryName, VIN: vinDiesel}).Times(1).Return(nil)
	ds.EXPECT().SendFingerprintData(gomock.Any()).Times(1).Return(nil)

	err := ls.Fingerprint()

	assert.NoError(t, err)
}

func Test_loggerService_VINLoggers_noVINResponse(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"
	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	lss := mock_loggers.NewMockLoggerSettingsService(mockCtrl)

	pidl := mock_loggers.NewMockPIDLogger(mockCtrl)
	vsd := mock_gateways.NewMockVehicleSignalDecodingAPIService(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	ls := NewLoggerService(unitID, vl, pidl, ds, lss, vsd, logger)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))
	// mock eth addr
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce839613fffba74279579268"}`))

	// nil settings, eg. first time it runs, incompatiible vehicle
	lss.EXPECT().ReadVINConfig().Times(1).Return(nil, fmt.Errorf("error reading file: open /tmp/logger-settings.json: no such file or directory"))
	noVinErr := fmt.Errorf("response contained an invalid vin")
	vl.EXPECT().GetVIN(unitID, nil).Times(1).Return(nil, noVinErr)
	lss.EXPECT().WriteVINConfig(loggers.VINLoggerSettings{
		VINQueryName:            "",
		VINLoggerVersion:        loggers.VINLoggerVersion,
		VINLoggerFailedAttempts: 1,
		VIN:                     "",
	}).Return(nil)
	ds.EXPECT().SendErrorPayload(gomock.Any(), gomock.Any()).Times(1).Return(nil)

	err := ls.Fingerprint()

	assert.NoError(t, err)
}

func Test_loggerService_VINLoggers_noVINResponseAndAttemptsExceeded(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"
	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	lss := mock_loggers.NewMockLoggerSettingsService(mockCtrl)
	pidl := mock_loggers.NewMockPIDLogger(mockCtrl)
	vsd := mock_gateways.NewMockVehicleSignalDecodingAPIService(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	ls := NewLoggerService(unitID, vl, pidl, ds, lss, vsd, logger)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))
	// mock eth addr
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce839613fffba74279579268"}`))

	// nil settings, eg. first time it runs, incompatiible vehicle
	lss.EXPECT().ReadVINConfig().Times(1).Return(&loggers.VINLoggerSettings{
		VINQueryName:            "",
		VINLoggerVersion:        loggers.VINLoggerVersion,
		VINLoggerFailedAttempts: 5,
	}, nil)

	err := ls.Fingerprint()

	assert.Error(t, err)
}
