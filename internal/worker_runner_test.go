package internal

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	mock_loggers "github.com/DIMO-Network/edge-network/internal/loggers/mocks"
	"github.com/DIMO-Network/edge-network/internal/models"
	mock_network "github.com/DIMO-Network/edge-network/internal/network/mocks"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	"net/http"
	"os"
	"testing"
	_ "time"
)

func TestWorkerRunner_Run(t *testing.T) {
	// mock data-sender and others deps
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
	// mock eth addr
	ethPath := fmt.Sprintf("/dongle/%s/execute_raw", unitID)
	httpmock.RegisterResponder(http.MethodPost, autoPiBaseURL+ethPath,
		httpmock.NewStringResponder(200, `{"value": "b794f5ea0ba39494ce839613fffba74279579268"}`))

	// Initialize workerRunner here with mocked dependencies
	wr := &workerRunner{
		loggerSettingsSvc: ts,
		dataSender:        ds,
		deviceSettings:    &models.TemplateDeviceSettings{},
		fingerprintRunner: ls,
		unitID:            unitID,
		pids:              &models.TemplatePIDs{},
	}

	// expect
	vinQueryName := "vin_7DF_09_02"
	ts.EXPECT().ReadVINConfig().Times(1).Return(nil, fmt.Errorf("error reading file: open /tmp/logger-settings.json: no such file or directory"))
	vl.EXPECT().GetVIN(unitID, nil).Times(1).Return(&loggers.VINResponse{VIN: "TESTVIN123", Protocol: "6", QueryName: vinQueryName}, nil)
	ts.EXPECT().WriteVINConfig(models.VINLoggerSettings{VINQueryName: vinQueryName, VIN: "TESTVIN123"}).Times(1).Return(nil)
	ds.EXPECT().SendFingerprintData(gomock.Any()).Times(1).Return(nil)

	// Run the method
	s := wr.createDeviceEvent("") // Since it's a loop, consider running it in a goroutine

	// pretty printing
	prettyJSON, err := json.MarshalIndent(s, "", "    ") // Prefix "", Indent "    "
	if err != nil {
		//log.Fatal("Error marshalling JSON:", err)
	}
	// Print the pretty JSON
	fmt.Println(string(prettyJSON))
	// Simulate a condition to stop the loop if necessary, for example by setting a condition or sending a signal

	// Assertions

}
