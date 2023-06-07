package loggers

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

//go:generate mockgen -source logger_settings.go -destination mocks/logger_settings_mock.go
type LoggerSettingsService interface {
	ReadConfig() (*LoggerSettings, error)
	WriteConfig(settings LoggerSettings) error
}

type loggerSettingsService struct {
	mu sync.Mutex
}

func NewLoggerSettingsService() LoggerSettingsService {
	return &loggerSettingsService{}
}

const filePath = "/tmp/logger-settings.json"

func (lcs *loggerSettingsService) ReadConfig() (*LoggerSettings, error) {
	lcs.mu.Lock()
	defer lcs.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &LoggerSettings{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

// WriteConfig writes the config file in json format to tmp folder, overwriting anything already existing
func (lcs *loggerSettingsService) WriteConfig(settings LoggerSettings) error {
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

type LoggerSettings struct {
	VINQueryName            string `json:"vin_query_name"`
	VINLoggerVersion        int    `json:"vin_logger_version"`
	VINLoggerFailedAttempts int    `json:"vin_logger_failed_attempts"`
}
