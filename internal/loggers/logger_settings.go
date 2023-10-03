package loggers

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/DIMO-Network/edge-network/internal/constants"
)

//go:generate mockgen -source logger_settings.go -destination mocks/logger_settings_mock.go
type LoggerSettingsService interface {
	ReadVINConfig() (*VINLoggerSettings, error)
	WriteVINConfig(settings VINLoggerSettings) error

	ReadPIDsConfig() (*PIDLoggerSettings, error)
	WritePIDsConfig(settings PIDLoggerSettings) error
}

// loggerSettingsService wraps reading and writing different configurations
type loggerSettingsService struct {
	mu sync.Mutex
}

func NewLoggerSettingsService() LoggerSettingsService {
	return &loggerSettingsService{}
}

func (lcs *loggerSettingsService) ReadVINConfig() (*VINLoggerSettings, error) {
	data, err := lcs.readConfig(constants.VINLoggerFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &VINLoggerSettings{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

func (lcs *loggerSettingsService) WriteVINConfig(settings VINLoggerSettings) error {
	err := lcs.writeConfig(constants.VINLoggerFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (lcs *loggerSettingsService) ReadPIDsConfig() (*PIDLoggerSettings, error) {
	data, err := lcs.readConfig(constants.PIDConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &PIDLoggerSettings{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

func (lcs *loggerSettingsService) WritePIDsConfig(settings PIDLoggerSettings) error {
	err := lcs.writeConfig(constants.PIDConfigFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (lcs *loggerSettingsService) readConfig(filePath string) ([]byte, error) {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}

	return data, nil
}

// WriteConfig writes the config file in json format to tmp folder, overwriting anything already existing
func (lcs *loggerSettingsService) writeConfig(filePath string, settings interface{}) error {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()
	// Open the file for writing (create if it doesn't exist)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %s", err)
	}
	defer file.Close()

	// Write data to the file
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("error writing file: %s", err)
	}
	return nil
}

type VINLoggerSettings struct {
	// VIN is whatever VIN we last were able to get from the vehicle
	VIN                     string `json:"vin"`
	VINQueryName            string `json:"vin_query_name"`
	VINLoggerVersion        int    `json:"vin_logger_version"`
	VINLoggerFailedAttempts int    `json:"vin_logger_failed_attempts"`
}

type PIDLoggerSettings struct {
	PidURL  string                  `json:"pidUrl"`
	Version string                  `json:"version"`
	PIDs    []PIDLoggerItemSettings `json:"items"`
}

type PIDLoggerItemSettings struct {
	Name     string `json:"name"`
	Formula  string `json:"formula"`
	Protocol string `json:"protocol"`
	Header   uint32 `json:"header"`
	PID      uint32 `json:"PID"`
	Mode     uint32 `json:"mode"`
	Interval int    `json:"interval"`
}
