package loggers

import (
	"fmt"
	"github.com/rs/zerolog"
	"sync"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/queue"
	"github.com/google/uuid"
)

//go:generate mockgen -source pid_logger.go -destination mocks/pid_logger_mock.go
type PIDLogger interface {
	ExecutePID(header, mode, pid, formula, protocol string) (err error)
}

type pidLogger struct {
	mu           sync.Mutex
	unitID       uuid.UUID
	storageQueue queue.StorageQueue
	logger       zerolog.Logger
}

func NewPIDLogger(unitID uuid.UUID, storageQueue queue.StorageQueue, logger zerolog.Logger) PIDLogger {
	return &pidLogger{unitID: unitID, storageQueue: storageQueue, logger: logger}
}

func (vl *pidLogger) ExecutePID(header, mode, pid, formula, protocol string) (err error) {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	cmd := fmt.Sprintf(`obd.query vin %s mode=%s pid=%s %s force=True protocol=%s`,
		header, mode, pid, formula, protocol)

	req := api.ExecuteRawRequest{Command: cmd}
	url := fmt.Sprintf("/dongle/%s/execute_raw", vl.unitID)

	var resp api.ExecuteRawResponse

	err = api.ExecuteRequest("POST", url, req, &resp)
	if err != nil {
		vl.logger.Fatal().Err(err).Msg("failed to execute POST request")
		return err
	}
	vl.logger.Info().Msgf("received PID response value: %s \n", resp.Value) // for debugging - will want this to validate.

	vl.storageQueue.Enqueue(resp.Value)

	return
}
