package internal

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
	return nil
}

// isOkToScan checks if the power status and other heuristics to determine if ok to start Open CAN scanning and PID requests
func (ls *loggerService) isOkToScan() bool {
	return false
}

type LoggerConfig struct {
	// SignalName name of signal to be published over mqtt
	SignalName string
	// Interval is how often to run. 0 means only on start
	Interval int32
	// Function to call to get the data from the vehicle
	ScanFunc func() any
}
