package internal

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/hooks"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/rs/zerolog"
	"sync"
	"time"
)

// SignalFrameDumpQueue part of the CAN frame dumps project for python to DBC formulas. similar to above but for storing can dumps
type SignalFrameDumpQueue struct {
	signalFrames map[string][]models.SignalCanFrameDump
	// we will check this to decide when to send the signalFrames over mqtt
	lastEnqueued time.Time
	jobDone      bool
	sync.RWMutex
	logger     zerolog.Logger
	dataSender network.DataSender
}

func NewSignalFrameDumpQueue(logger zerolog.Logger, sender network.DataSender) *SignalFrameDumpQueue {
	// todo set signal dump queue `jobDone` to true if we have something on disk
	// leverage the config service to read something from disk and check
	return &SignalFrameDumpQueue{signalFrames: make(map[string][]models.SignalCanFrameDump), logger: logger,
		dataSender: sender}
}

func (scf *SignalFrameDumpQueue) Enqueue(signal models.SignalCanFrameDump) {
	scf.Lock()
	defer scf.Unlock()

	scf.lastEnqueued = time.Now()
	scf.signalFrames[signal.Name] = append(scf.signalFrames[signal.Name], signal)
}

func (scf *SignalFrameDumpQueue) Dequeue() []models.SignalCanFrameDump {
	scf.Lock()
	defer scf.Unlock()
	// iterate over the signals map and return just the []models.SignalsData
	var data []models.SignalCanFrameDump
	for _, signalMap := range scf.signalFrames {
		for _, frame := range signalMap {
			data = append(data, frame)
		}
	}
	// empty the data after dequeue
	scf.signalFrames = map[string][]models.SignalCanFrameDump{}

	return data
}

// wantMoreCanFrameDump true if we want more CAN dumps for this pid. Checks underlying data structure that tracks
func (scf *SignalFrameDumpQueue) wantMoreCanFrameDump(signalName string) bool {
	const maxCanDumpFrames = 2

	if s, ok := scf.signalFrames[signalName]; ok {
		if len(s) < maxCanDumpFrames {
			return true
		}
	} else {
		return true
	}

	return false
}

// ShouldCaptureReq determines if we should capture the current request for a dump. Should be false once we're done
func (scf *SignalFrameDumpQueue) ShouldCaptureReq(request models.PIDRequest) bool {
	return !scf.jobDone && request.FormulaType() == models.Python && scf.wantMoreCanFrameDump(request.Name)
}

// SenderWorker checks on a long interval for signal frames that need to be dequeued and sent over MQTT
func (scf *SignalFrameDumpQueue) SenderWorker() {
	// todo future: what if no custom python PIDs - pretty common
	loopCount := 0
	for !scf.jobDone {
		if scf.lastEnqueued.Before(time.Now().Add(3*time.Minute)) &&
			len(scf.signalFrames) > 0 {
			bytes, err := json.Marshal(scf.Dequeue())
			if err != nil {
				scf.logger.Err(err).Msg("failed to marshal signalDumpFrames")
			}
			err = scf.dataSender.SendCanDumpData(bytes)
			if err != nil {
				hooks.LogError(scf.logger, err, fmt.Sprintf("failed to send canDumpData for custom pids.data length: %d", len(bytes)),
					hooks.WithThresholdWhenLogMqtt(5), hooks.WithStopLogAfter(3))
			}
			scf.logger.Info().Msgf("successfully sent signalDumpFrames. data length: %d", len(bytes))
			scf.jobDone = true // persist this somewhere?
		}
		time.Sleep(30 * time.Second)
		loopCount++
		// control for too many loops
		if loopCount > 30 {
			scf.jobDone = true
		}
	}
}
