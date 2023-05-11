package internal

import (
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/google/uuid"
)

type LoggerService interface {
	StartLoggers() error
}

type loggerService struct {
}

func NewLoggerService() LoggerService {
	return &loggerService{}
}

// StartLoggers checks if ok to start scanning the vehicle and then according to configuration scans and sends data periodically
func (ls *loggerService) StartLoggers() error {
	// todo this should only start if the vehicle has already been paired to the autopi

	return nil
}

// isOkToScan checks if the power status and other heuristics to determine if ok to start Open CAN scanning and PID requests
func (ls *loggerService) isOkToScan() bool {
	return false
}

func (ls *loggerService) getLoggerConfigs() []LoggerConfig {
	return []LoggerConfig{
		{
			SignalName: "vin",
			Interval:   0,
			ScanFunc:   commands.GetVIN,
		},
	}
}

type LoggerConfig struct {
	// SignalName name of signal to be published over mqtt
	SignalName string
	// Interval is how often to run. 0 means only on start
	Interval int32
	// Function to call to get the data from the vehicle
	ScanFunc func(uuid2 uuid.UUID) (*commands.VINResponse, error)
}
