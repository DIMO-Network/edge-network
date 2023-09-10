package loggers

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"sync"
)

//go:generate mockgen -source pid_logger.go -destination mocks/pid_logger_mock.go
type PIDLogger interface {
	ExecutePID() (err error)
}

type pidLogger struct {
	mu                sync.Mutex
	loggerSettingsSvc LoggerSettingsService
	unitID            uuid.UUID
}

func NewPIDLogger(loggerSettingsSvc LoggerSettingsService, id uuid.UUID) PIDLogger {
	return &pidLogger{loggerSettingsSvc: loggerSettingsSvc, unitID: id}
}

func (vl *pidLogger) ExecutePID() (err error) {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	config, err := vl.loggerSettingsSvc.ReadPIDsConfig()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(`obd.query vin %s mode=%s pid=%s %s force=True protocol=%s`,
		config.Header, config.Mode, config.PID, config.Formula, config.Protocol)

	req := api.ExecuteRawRequest{Command: cmd}
	url := fmt.Sprintf("/dongle/%s/execute_raw", vl.unitID)

	var resp api.ExecuteRawResponse

	err = api.ExecuteRequest("POST", url, req, &resp)
	if err != nil {
		log.WithError(err).Error("failed to execute POST request")
		return err
	}
	log.Infof("received PID response value: %s \n", resp.Value) // for debugging - will want this to validate.

	return
}
