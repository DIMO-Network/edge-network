package internal

import (
	"encoding/json"
	"fmt"
	"os"
)

type LoggerSettingsService interface {
	ReadConfig() (*LoggerSettings, error)
	WriteConfig(settings LoggerSettings) error
}

type loggerSettingsService struct {
}

func NewLoggerSettingsService() LoggerSettingsService {
	return &loggerSettingsService{}
}

const filePath = "/tmp/logger-settings.json"

func (lcs *loggerSettingsService) ReadConfig() (*LoggerSettings, error) {
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

func (lcs *loggerSettingsService) WriteConfig(settings LoggerSettings) error {
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
	VINQueryName string `json:"vin_query_name"`
}
