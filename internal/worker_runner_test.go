package internal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
	_ "time"

	"github.com/DIMO-Network/edge-network/internal/hooks"

	"github.com/DIMO-Network/edge-network/internal/loggers"
	mock_loggers "github.com/DIMO-Network/edge-network/internal/loggers/mocks"
	"github.com/DIMO-Network/edge-network/internal/models"
	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_workerRunner_NonObd(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	_, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)

	const autoPiBaseURL = "http://192.168.4.1:9000"
	wfPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+wfPath,
		httpmock.NewStringResponder(200, `{"wpa_state": "COMPLETED", "ssid": "test", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// mock obd resp
	locPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+locPath,
		httpmock.NewStringResponder(200, `{"lat": 37.7749, "lon": -122.4194, "_stamp": "2024-02-29T17:17:30.534861"}`))

	// Initialize workerRunner here with mocked dependencies
	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)

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

	_, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().Return(false)

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
	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests

	// then
	_, _ = wr.isOkToQueryOBD()
	wr.queryOBD(nil)

	// verify
	assert.Equal(t, "fuellevel", wr.signalsQueue.signals[0].Name)
	assert.Equal(t, 2, len(wr.signalsQueue.signals))
	assert.Equal(t, 2, len(wr.signalsQueue.lastTimeChecked))
}

func Test_workerRunner_Obd_With_Python_Formula(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	_, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().Return(false)

	const autoPiBaseURL = "http://192.168.4.1:9000"
	// mock obd resp
	obdPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+obdPath,
		httpmock.NewStringResponder(200, `{"value": 17.92, "_stamp": "2024-02-29T17:17:30.534861"}`))
	obdPath1 := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+obdPath1,
		httpmock.NewStringResponder(200, `{"value": 18.95, "_stamp": "2024-02-29T17:17:30.534861"}`))
	obdPath2 := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+obdPath2,
		httpmock.NewStringResponder(200, `{"value": "17.00", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "foo",
			IntervalSeconds: 10,
			Formula:         "python:bytes_to_int(messages[0].data[-2:]) * 0.001",
		},
		{
			Name:            "boo",
			IntervalSeconds: 5,
			Formula:         "python:bytes_to_int(messages[0].data[-2:]) * 0.001",
		},
		{
			Name:            "baz",
			IntervalSeconds: 5,
			Formula:         "python:bytes_to_int(messages[0].data[-2:]) * 0.001",
		},
	}

	// Initialize workerRunner here with mocked dependencies
	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests

	// then
	wr.queryOBD(nil)

	// verify
	assert.Equal(t, "foo", wr.signalsQueue.signals[0].Name)
	assert.Equal(t, 3, len(wr.signalsQueue.signals))
	assert.Equal(t, 3, len(wr.signalsQueue.lastTimeChecked))
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

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().Return(false)

	// mock powerstatus resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	expectOnMocks(ts, vl, unitID, ds, 0)

	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 60,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests

	// then
	_, powerStatus := wr.isOkToQueryOBD()
	wr.queryOBD(nil)
	err := wr.fingerprintRunner.FingerprintSimple(powerStatus)
	wifi, wifiErr, location, locationErr, _, _ := wr.queryNonObd("ec2x")
	s := wr.composeDeviceEvent(powerStatus, locationErr, location, wifiErr, wifi)

	// verify
	assert.Nil(t, err)
	assert.Equal(t, 13.3, s.Device.BatteryVoltage)
	assert.Equal(t, "fuellevel", s.Vehicle.Signals[0].Name)
	assert.Equal(t, 0, len(wr.signalsQueue.signals), "signals slice should be empty after composing device event")
	assert.Equal(t, 1, len(wr.signalsQueue.lastTimeChecked), "signals cache should have 1 entry after composing device event")
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

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called twice with expected payload
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, "fuellevel", data.Vehicle.Signals[0].Name)
		assert.Equal(t, 9, len(data.Vehicle.Signals))
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 9, len(data.Vehicle.Signals))
	}).Return(nil)

	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
		assert.NotNil(t, data.Longitude)
	}).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 6,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 5 * time.Second
	wr.stop = make(chan bool)

	// then
	go wr.Run()
	time.Sleep(10 * time.Second)
	wr.Stop()
}

// test for both obd and non-obd and location which executes concurrently
func Test_workerRunner_Run_withLocationQuery(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock location data
	locPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+locPath,
		func(req *http.Request) (*http.Response, error) {
			// Read the request body
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), err
			}
			// Convert the body bytes to string
			bodyString := string(bodyBytes)

			// Match the request body
			if strings.Contains(bodyString, "config.get modem") {
				return httpmock.NewStringResponse(200, `{"response": "ok"}`), nil
			} else if strings.Contains(bodyString, "ec2x.gnss_location") {
				return httpmock.NewStringResponse(200, `{"lat": 42.270118333333336 , "lon": -71.50163833333333}`), nil
			} else if strings.Contains(bodyString, "obd.query") {
				return httpmock.NewStringResponse(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`), nil
			} else if strings.Contains(bodyString, "power.status") {
				return httpmock.NewStringResponse(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`), nil
			}
			// If the request body does not match, return an error response
			return httpmock.NewStringResponse(400, `{"error": "invalid request body"}`), nil
		},
	)

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called twice with expected payload
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.True(t, len(data.Vehicle.Signals) > 10)
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.True(t, len(data.Vehicle.Signals) > 40, "should have more signals after second data send")
	}).Return(nil)

	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
		assert.NotNil(t, data.Longitude)
	}).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 6,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 5 * time.Second
	// since location consists from 4 signals, we should have more than 40 signals in the 5 sec interval
	wr.deviceSettings.LocationFrequencySecs = 0.5
	wr.stop = make(chan bool)

	// then
	go wr.Run()
	time.Sleep(10 * time.Second)
	wr.Stop()
}

func Test_workerRunner_Run_sendSameSignalMultipleTimes(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called once with multiple fuel level signals
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, "fuellevel", data.Vehicle.Signals[0].Name)
		assert.Equal(t, 8, len(data.Vehicle.Signals))
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, "fuellevel", data.Vehicle.Signals[0].Name)
		assert.Equal(t, 9, len(data.Vehicle.Signals))
	}).Return(nil)

	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
	}).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 3,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 10 * time.Second
	wr.stop = make(chan bool)

	// then the data sender should be called twice
	go wr.Run()
	time.Sleep(15 * time.Second)
	wr.Stop()
}

func Test_workerRunner_Run_donotSendIfNoSignals(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock wifi resp
	wfPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+wfPath,
		httpmock.NewStringResponder(200, `{"wpa_state": "DISCONNECTED", "ssid": "", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// mock location resp
	locPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+locPath,
		httpmock.NewStringResponder(500, `{"error":"Error on query gps"}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(500, `{"error":"Failed to calculate formula: invalid syntax (<string>, line 1)"}`))

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called once with multiple fuel level signals
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(0).Return(nil)

	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
	}).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 3,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 10 * time.Second
	wr.stop = make(chan bool)

	// then the data sender should be called twice
	go wr.Run()
	time.Sleep(15 * time.Second)
	wr.Stop()
}

func Test_workerRunner_Run_sendSignalsWithDifferentInterval(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)
	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called once with multiple fuel level signals
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 11, len(data.Vehicle.Signals))
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 10, len(data.Vehicle.Signals))
	}).Return(nil)

	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
	}).Return(nil)

	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 3,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
		{
			Name:            "rpm",
			IntervalSeconds: 5,
			Formula:         "dbc: 31|16@0+ (0.25,0) [0|16383.75] \"%\"",
		},
		{
			Name:            "foo",
			IntervalSeconds: 30,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
		{
			Name:            "baz",
			IntervalSeconds: 0,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	// Initialize workerRunner here with mocked dependencies
	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 10 * time.Second
	wr.stop = make(chan bool)

	// then the data sender should be called twice
	go wr.Run()
	time.Sleep(15 * time.Second)
	wr.Stop()
}

func Test_workerRunner_Run_failedToQueryPidTooManyTimes(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+path,
		func(req *http.Request) (*http.Response, error) {
			// Read the request body
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), err
			}
			// Convert the body bytes to string
			bodyString := string(bodyBytes)

			// Match the request body
			if strings.Contains(bodyString, "obd.query fuellevel") {
				return httpmock.NewStringResponse(500, `{"error":"Failed to calculate formula: invalid syntax (<string>, line 1)"}`), nil
			} else if strings.Contains(bodyString, "obd.query foo") {
				return httpmock.NewStringResponse(500, `{"error":"Failed to calculate formula: invalid syntax (<string>, line 1)"}`), nil
			}
			return httpmock.NewStringResponse(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`), nil
		},
	)

	expectOnMocks(ts, vl, unitID, ds, 1)

	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 1,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
		{
			Name:            "foo",
			IntervalSeconds: 0,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 10 * time.Second
	wr.stop = make(chan bool)
	wr.logger = zerolog.New(os.Stdout).With().Timestamp().Str("app", "edge-network").Logger()

	// assert data sender is called without fuel level signal
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(3).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 7, len(data.Vehicle.Signals))
	}).Return(nil)
	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(3).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
	}).Return(nil)
	// then the data sender should be called twice
	go wr.Run()
	time.Sleep(25 * time.Second)
	assert.Equal(t, 11, wr.signalsQueue.failureCount["fuellevel"])
	assert.Equal(t, 11, wr.signalsQueue.failureCount["foo"])
	wr.Stop()
}

func Test_workerRunner_Run_failedToQueryPidButRecover(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock obd resp
	path := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	var count int
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+path,
		func(req *http.Request) (*http.Response, error) {
			// Read the request body
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), err
			}
			// Convert the body bytes to string
			bodyString := string(bodyBytes)

			// Match the request body
			if strings.Contains(bodyString, "obd.query fuellevel") && count < 10 {
				count++
				return httpmock.NewStringResponse(500, `{"error":"Failed to calculate formula: invalid syntax (<string>, line 1)"}`), nil
			} else if strings.Contains(bodyString, "obd.query foo") && count < 10 {
				count++
				return httpmock.NewStringResponse(500, `{"error":"Failed to calculate formula: invalid syntax (<string>, line 1)"}`), nil
			}
			return httpmock.NewStringResponse(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`), nil
		},
	)

	expectOnMocks(ts, vl, unitID, ds, 1)

	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 1,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
		{
			Name:            "foo",
			IntervalSeconds: 0,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 10 * time.Second
	wr.stop = make(chan bool)
	wr.logger = zerolog.New(os.Stdout).With().Timestamp().Str("app", "edge-network").Logger()
	fh := hooks.NewLogRateLimiterHook(ds)
	wr.logger = wr.logger.Hook(&hooks.LogHook{DataSender: ds}).Hook(fh)

	// assert data sender is called without fuel level signal
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 7, len(data.Vehicle.Signals))
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 9, len(data.Vehicle.Signals))
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 12, len(data.Vehicle.Signals))
		found := false
		for _, signal := range data.Vehicle.Signals {
			if signal.Name == "foo" {
				found = true
				break
			}
		}
		assert.Falsef(t, found, "foo signal should not be present in the signals")
	}).Return(nil)
	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(3).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
	}).Return(nil)
	// then the data sender should be called twice
	go wr.Run()
	time.Sleep(25 * time.Second)
	// failure counter should be reset after success query
	assert.Equal(t, 0, wr.signalsQueue.failureCount["fuellevel"])
	assert.Equal(t, 0, wr.signalsQueue.failureCount["foo"])
	wr.Stop()
}

func Test_workerRunner_RunWithNotEnoughVoltage(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	var callCount = 0
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		func(req *http.Request) (*http.Response, error) {
			// Read the request body
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), err
			}
			// Convert the body bytes to string
			bodyString := string(bodyBytes)

			if strings.Contains(bodyString, "power.status") {
				callCount++
				var resp string
				switch callCount {
				case 1:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`
				case 2:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`
				case 3:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 12.3}}}`
				case 4:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 12.3}}}`
				case 7:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`
				case 8:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`
				default:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 11.3}}}`
				}
				return httpmock.NewStringResponse(200, resp), nil
			}
			return httpmock.NewStringResponse(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`), nil
		},
	)

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called twice with expected payload
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(2).Return(nil)
	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 6,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 5 * time.Second
	wr.stop = make(chan bool)
	wr.deviceSettings.MinVoltageOBDLoggers = 13.3
	var buf bytes.Buffer
	wr.logger = zerolog.New(&buf).With().Timestamp().Str("app", "edge-network").Logger()
	fh := hooks.NewLogRateLimiterHook(ds)
	wr.logger = wr.logger.Hook(&hooks.LogHook{DataSender: ds}).Hook(fh)

	// then
	go wr.Run()
	time.Sleep(10 * time.Second)
	wr.Stop()

	// verify
	assert.Contains(t, buf.String(), "voltage not enough to query obd: 12.3")
	count := strings.Count(buf.String(), "voltage not enough to query obd: 12.3")
	assert.Equal(t, 1, count)
}

func Test_workerRunner_RunWithNotEnoughVoltage2(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	var callCount = 0
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		func(req *http.Request) (*http.Response, error) {
			// Read the request body
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), err
			}
			// Convert the body bytes to string
			bodyString := string(bodyBytes)

			if strings.Contains(bodyString, "power.status") {
				callCount++
				var resp string
				switch callCount {
				case 1:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`
				case 2:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`
				case 3:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 11.3}}}`
				case 4:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 11.3}}}`
				case 7:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 11.3}}}`
				case 8:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 11.3}}}`
				default:
					resp = `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.4}}}`
				}
				return httpmock.NewStringResponse(200, resp), nil
			}
			return httpmock.NewStringResponse(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`), nil
		},
	)

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called twice with expected payload
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(2).Return(nil)
	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 6,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 5 * time.Second
	wr.stop = make(chan bool)
	wr.deviceSettings.MinVoltageOBDLoggers = 13.3
	var buf bytes.Buffer
	wr.logger = zerolog.New(&buf).With().Timestamp().Str("app", "edge-network").Logger()
	fh := hooks.NewLogRateLimiterHook(ds)
	wr.logger = wr.logger.Hook(&hooks.LogHook{DataSender: ds}).Hook(fh)

	// then
	go wr.Run()
	time.Sleep(10 * time.Second)
	wr.Stop()

	// verify
	assert.Contains(t, buf.String(), "voltage not enough to query obd: 11.3")
	count := strings.Count(buf.String(), "voltage not enough to query obd")
	assert.Equal(t, 1, count)
}

// This is test for the case when the location query fails and we want to rate limit logs
func Test_workerRunner_RunWithCantQueryLocation(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		func(req *http.Request) (*http.Response, error) {
			// Read the request body
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return httpmock.NewStringResponse(500, ""), err
			}
			// Convert the body bytes to string
			bodyString := string(bodyBytes)

			if strings.Contains(bodyString, "power.status") {
				resp := `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`
				return httpmock.NewStringResponse(200, resp), nil
			}
			return httpmock.NewStringResponse(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`), nil
		},
	)
	// mock location resp
	locPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+locPath,
		httpmock.NewStringResponder(500, `{"error":"Error on query gps"}`))

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called twice with expected payload
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(2).Return(nil)
	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 6,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 5 * time.Second
	wr.stop = make(chan bool)
	wr.deviceSettings.MinVoltageOBDLoggers = 13.3
	var buf bytes.Buffer
	wr.logger = zerolog.New(&buf).With().Timestamp().Str("app", "edge-network").Logger()
	fh := hooks.NewLogRateLimiterHook(ds)
	wr.logger = wr.logger.Hook(&hooks.LogHook{DataSender: ds}).Hook(fh)

	// then
	go wr.Run()
	time.Sleep(10 * time.Second)
	wr.Stop()

	// verify
	assert.Contains(t, buf.String(), "Error on query gps")
	count := strings.Count(buf.String(), "failed to get gps location")
	assert.Equal(t, 1, count)
}

func Test_workerRunner_FilterWiFiWhenDisconnected(t *testing.T) {
	// when
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	const autoPiBaseURL = "http://192.168.4.1:9000"

	unitID := uuid.New()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	vl, ds, ts, dbcS, ls := mockComponents(mockCtrl, unitID)
	dbcS.EXPECT().UseNativeScanLogger().AnyTimes().Return(false)

	// mock power status resp
	psPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+psPath,
		httpmock.NewStringResponder(200, `{"spm": {"last_trigger": {"up": "volt_change"}, "battery": {"voltage": 13.3}}}`))

	// mock wifi resp
	wfPath := fmt.Sprintf("/dongle/%s/execute_raw/", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+wfPath,
		httpmock.NewStringResponder(200, `{"wpa_state": "DISCONNECTED", "ssid": "", "_stamp": "2024-02-29T17:17:30.534861"}`))

	// mock obd resp
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "7e803412f6700000000", "_stamp": "2024-02-29T17:17:30.534861"}`))

	expectOnMocks(ts, vl, unitID, ds, 1)

	// assert data sender is called twice with expected payload
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, "fuellevel", data.Vehicle.Signals[0].Name)
		assert.Equal(t, 6, len(data.Vehicle.Signals))
	}).Return(nil)
	ds.EXPECT().SendDeviceStatusData(gomock.Any()).Times(1).Do(func(data models.DeviceStatusData) {
		assert.Equal(t, 5, len(data.Vehicle.Signals))
	}).Return(nil)

	ds.EXPECT().SendDeviceNetworkData(gomock.Any()).Times(2).Do(func(data models.DeviceNetworkData) {
		assert.NotNil(t, data.Cell)
		assert.NotNil(t, data.Longitude)
	}).Return(nil)
	// Initialize workerRunner here with mocked dependencies
	requests := []models.PIDRequest{
		{
			Name:            "fuellevel",
			IntervalSeconds: 6,
			Formula:         "dbc:31|8@0+ (0.392156862745098,0) [0|100] \"%\"",
		},
	}

	wr := createWorkerRunner(ts, ds, dbcS, ls, unitID)
	wr.pids.Requests = requests
	wr.sendPayloadInterval = 5 * time.Second
	wr.stop = make(chan bool)

	// then
	go wr.Run()
	time.Sleep(10 * time.Second)
	wr.Stop()
}

func mockComponents(mockCtrl *gomock.Controller, unitID uuid.UUID) (*mock_loggers.MockVINLogger, *mock_network.MockDataSender, *mock_loggers.MockTemplateStore, *mock_loggers.MockDBCPassiveLogger, FingerprintRunner) {
	vl := mock_loggers.NewMockVINLogger(mockCtrl)
	ds := mock_network.NewMockDataSender(mockCtrl)
	ts := mock_loggers.NewMockTemplateStore(mockCtrl)
	dbcS := mock_loggers.NewMockDBCPassiveLogger(mockCtrl)

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	ts.EXPECT().ReadVINConfig().Times(1).Return(nil, fmt.Errorf("error reading file: open /tmp/logger-settings.json: no such file or directory"))

	ls := NewFingerprintRunner(unitID, vl, ds, ts, logger)
	return vl, ds, ts, dbcS, ls
}

func expectOnMocks(ts *mock_loggers.MockTemplateStore, vl *mock_loggers.MockVINLogger, unitID uuid.UUID, ds *mock_network.MockDataSender, readVinNum int) {
	vinQueryName := "vin_7DF_09_02"
	ts.EXPECT().ReadVINConfig().Times(readVinNum).Return(nil, fmt.Errorf("error reading file: open /tmp/logger-settings.json: no such file or directory"))
	vl.EXPECT().GetVIN(unitID, nil).Times(1).Return(&loggers.VINResponse{VIN: "TESTVIN123", Protocol: "6", QueryName: vinQueryName}, nil)
	ts.EXPECT().WriteVINConfig(models.VINLoggerSettings{VINQueryName: vinQueryName, VIN: "TESTVIN123"}).Times(1).Return(nil)
	ds.EXPECT().SendFingerprintData(gomock.Any()).Times(1).Return(nil)
}

func createWorkerRunner(ts *mock_loggers.MockTemplateStore, ds *mock_network.MockDataSender, dbcS *mock_loggers.MockDBCPassiveLogger, ls FingerprintRunner, unitID uuid.UUID) *workerRunner {
	wr := &workerRunner{
		loggerSettingsSvc: ts,
		dataSender:        ds,
		deviceSettings:    &models.TemplateDeviceSettings{},
		fingerprintRunner: ls,
		device: Device{
			UnitID: unitID,
		},
		pids:         &models.TemplatePIDs{Requests: nil, TemplateName: "test", Version: "1.0"},
		signalsQueue: &SignalsQueue{lastTimeChecked: make(map[string]time.Time), failureCount: make(map[string]int)},
		vehicleInfo: &models.VehicleInfo{
			TokenID: 12345,
			VehicleDefinition: models.VehicleDefinition{
				Make:  "Toyota",
				Model: "Corolla",
				Year:  2022,
			},
		},
		dbcScanner: dbcS,
	}
	return wr
}
