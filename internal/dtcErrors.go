package internal

import (
	"fmt"
	"time"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type DtcErrorsRunner interface {
	DtcErrors() error
	CurrentFailureCount() int
	//IncrementFailuresReached() int
}

type dtcErrorsRunner struct {
	unitID     uuid.UUID
	logger     zerolog.Logger
	dataSender network.DataSender
	// state tracking
	failureCount int
}

func NewDtcErrorsRunner(unitID uuid.UUID, dataSender network.DataSender, logger zerolog.Logger) DtcErrorsRunner {
	return &dtcErrorsRunner{
		unitID:       unitID,
		logger:       logger,
		dataSender:   dataSender,
		failureCount: 0,
	}
}

func (ls *dtcErrorsRunner) CurrentFailureCount() int {
	return ls.failureCount
}

// DtcErrors requests DTC scan from vehicle, if anything returned, sends a payload with signals data to device status
func (ls *dtcErrorsRunner) DtcErrors() error {
	codes, err := commands.GetDiagnosticCodes(ls.unitID, ls.logger)
	if err != nil {
		ls.failureCount++
		return errors.Wrap(err, fmt.Sprintf("failed to scan for dtc. fail count since boot: %d", ls.failureCount))
	}
	if len(codes) > 0 {
		// send the dtc in the signals using status topic. Seems pointless to send any other common data
		ts := time.Now().UTC().UnixMilli()
		s := models.DtcErrorsData{Vehicle: models.Vehicle{Signals: []models.SignalData{
			{
				Timestamp: ts,
				Name:      "dtcErrors",
				Value:     codes,
			},
		}}}
		return ls.dataSender.SendDeviceStatusData(s)
	}

	return nil
}
