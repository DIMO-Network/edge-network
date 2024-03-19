package internal

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	mock_loggers "github.com/DIMO-Network/edge-network/internal/loggers/mocks"
	"github.com/DIMO-Network/edge-network/internal/models"
	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"net/http"
	"os"
	"testing"
	"time"
	_ "time"
)

func Test_workerRunner_NonObd(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	ts := mock_loggers.NewMockTemplateStore(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()
	ls := NewFingerprintRunner(unitID, vl, ds, ts, logger)

	const autoPiBaseURL = "http://192.168.4.1:9000"
	wfPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+wfPath,
		httpmock.NewStringResponder(200, `{"wpa_state": "COMPLETED", "ssid": "test", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// mock obd resp
	locPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+locPath,
		httpmock.NewStringResponder(200, `{"lat": 37.7749, "lon": -122.4194, "_stamp": "2024-02-29T17:17:30.534861"}`))

	// Initialize workerRunner here with mocked dependencies
	wr := &workerRunner{
		loggerSettingsSvc: ts,
		dataSender:        ds,
		deviceSettings:    &models.TemplateDeviceSettings{},
		fingerprintRunner: ls,
		unitID:            unitID,
		pids:              &models.TemplatePIDs{Requests: nil, TemplateName: "test", Version: "1.0"},
		signalsQueue:      &SignalsQueue{lastTimeSent: make(map[string]time.Time)},
	}

	// then
	wifi, _, location, _, cellInfo, _ := wr.queryNonObd("ec2x")

	// verify
	assert.NotNil(t, cellInfo)
	assert.Equal(t, -122.4194, location.Longitude)
	assert.Equal(t, 37.7749, location.Latitude)
	assert.Equal(t, "test", wifi.SSID)
	assert.Equal(t, "COMPLETED", wifi.WPAState)
}

func Test_workerRunner_Obd(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	ts := mock_loggers.NewMockTemplateStore(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()
	ls := NewFingerprintRunner(unitID, vl, ds, ts, logger)

	const autoPiBaseURL = "http://192.168.4.1:9000"
	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	obdPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+obdPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))
	obdPath1 := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+obdPath1,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 10,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
		{
			Name:            "rpm",
			IntervalSeconds: 5,
			Formula:         "dbc: 31|16@0+ (0.25,0) [0|16383.75] \"%\"",
		},
	}

	// Initialize workerRunner here with mocked dependencies
	wr := &workerRunner{
		loggerSettingsSvc: ts,
		dataSender:        ds,
		deviceSettings:    &models.TemplateDeviceSettings{},
		fingerprintRunner: ls,
		unitID:            unitID,
		pids:              &models.TemplatePIDs{Requests: requests, TemplateName: "test", Version: "1.0"},
		signalsQueue:      &SignalsQueue{lastTimeSent: make(map[string]time.Time)},
	}

	// then
	_, _ = wr.isOkToQueryOBD()
	wr.queryOBD()

	// verify
	assert.Equal(t, "fuellevel", wr.signalsQueue.signals[0].Name)
	assert.Equal(t, 2, len(wr.signalsQueue.signals))
	assert.Equal(t, 2, len(wr.signalsQueue.lastTimeSent))
}

// test for both obd and non-obd signals which executes synchronously and not concurrently
func Test_workerRunner_OBD_and_NonObd(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	ts := mock_loggers.NewMockTemplateStore(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()
	ls := NewFingerprintRunner(unitID, vl, ds, ts, logger)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// todo mock location, network, wifi and others
	vinQueryName := "vin_7DF_09_02"
	ts.EXPECT().ReadVINConfig().Times(1).Return(nil, fmt.Errorf("error reading file: open /tmp/logger-settings.json: no such file or directory"))
	vl.EXPECT().GetVIN(unitID, nil).Times(1).Return(&loggers.VINResponse{VIN: "TESTVIN123", Protocol: "6", QueryName: vinQueryName}, nil)
	ts.EXPECT().WriteVINConfig(models.VINLoggerSettings{VINQueryName: vinQueryName, VIN: "TESTVIN123"}).Times(1).Return(nil)
	ds.EXPECT().SendFingerprintData(gomock.Any()).Times(1).Return(nil)

	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 60,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := &workerRunner{
		loggerSettingsSvc: ts,
		dataSender:        ds,
		deviceSettings:    &models.TemplateDeviceSettings{},
		fingerprintRunner: ls,
		unitID:            unitID,
		pids:              &models.TemplatePIDs{Requests: requests, TemplateName: "test", Version: "1.0"},
		signalsQueue:      &SignalsQueue{lastTimeSent: make(map[string]time.Time)},
	}

	// then
	_, powerStatus := wr.isOkToQueryOBD()
	wr.queryOBD()
	wr.fingerprintRunner.FingerprintSimple(powerStatus)
	wifi, wifiErr, location, locationErr, cellInfo, cellErr := wr.queryNonObd("ec2x")
	s := wr.composeDeviceEvent(powerStatus, locationErr, location, wifiErr, wifi, cellErr, cellInfo)

	// verify
	assert.NotNil(t, s.Network)
	assert.NotNil(t, s.Network.WiFi)
	assert.NotNil(t, s.Network.QMICellInfoResponse)
	assert.Equal(t, 13.3, s.Device.BatteryVoltage)
	assert.Equal(t, "fuellevel", s.Vehicle.Signals[0].Name)
	assert.Equal(t, 0, len(wr.signalsQueue.signals), "signals slice should be empty after composing device event")
	assert.Equal(t, 1, len(wr.signalsQueue.lastTimeSent), "signals cache should have 1 entry after composing device event")
}

// test for both obd and non-obd which executes concurrently as is in code
func Test_workerRunner_Run(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	ts := mock_loggers.NewMockTemplateStore(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()
	ls := NewFingerprintRunner(unitID, vl, ds, ts, logger)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// todo mock location, network, wifi and others
	vinQueryName := "vin_7DF_09_02"
	ts.EXPECT().ReadVINConfig().Times(2).Return(nil, fmt.Errorf("error reading file: open /tmp/logger-settings.json: no such file or directory"))
	vl.EXPECT().GetVIN(unitID, nil).Times(1).Return(&loggers.VINResponse{VIN: "TESTVIN123", Protocol: "6", QueryName: vinQueryName}, nil)
	ts.EXPECT().WriteVINConfig(models.VINLoggerSettings{VINQueryName: vinQueryName, VIN: "TESTVIN123"}).Times(1).Return(nil)
	ds.EXPECT().SendFingerprintData(gomock.Any()).Times(1).Return(nil)

	// assert data sender is called twice with expected payload
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, "fuellevel", data.Vehicle.Signals[0].Name)
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 0, len(data.Vehicle.Signals))
	}).Return(nil)

	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 6,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := &workerRunner{
		loggerSettingsSvc: ts,
		dataSender:        ds,
		deviceSettings:    &models.TemplateDeviceSettings{},
		fingerprintRunner: ls,
		unitID:            unitID,
		pids:              &models.TemplatePIDs{Requests: requests, TemplateName: "test", Version: "1.0"},
		signalsQueue:      &SignalsQueue{lastTimeSent: make(map[string]time.Time)},
		stop:              make(chan bool),
		obdInterval:       5 * time.Second,
	}

	go wr.Run()
	time.Sleep(10 * time.Second)
	wr.Stop()
}
