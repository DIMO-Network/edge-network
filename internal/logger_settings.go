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

func NewLoggerConfigService() LoggerSettingsService {
	return &loggerSettingsService{}
}

func (lcs *loggerSettingsService) ReadConfig() (*LoggerSettings, error) {

}

func (lcs *loggerSettingsService) WriteConfig(settings LoggerSettings) error {
	filePath := "/tmp/example.txt"

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
}

type LoggerSettings struct {
	VINQueryName string `json:"vin_query_name"`
}
