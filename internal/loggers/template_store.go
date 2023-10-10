package loggers

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/models"
	"os"
	"sync"
)

const (
	VINLoggerFile = "/tmp/vin-settings.json"
	PIDConfigFile = "/tmp/logger-pid-settings.json"
	TmpDirectory  = "/tmp/"
)

//go:generate mockgen -source template_store.go -destination mocks/template_store_mock.go
type TemplateStore interface {
	ReadVINConfig() (*models.VINLoggerSettings, error)
	WriteVINConfig(settings models.VINLoggerSettings) error

	ReadPIDsConfig() (*models.TemplatePIDs, error)
	WritePIDsConfig(settings models.TemplatePIDs) error

	ReadTemplateURLs() (*models.TemplateURLs, error)
	WriteTemplateURLs(settings models.TemplateURLs) error

	ReadTemplateDeviceSettings() (*models.TemplateDeviceSettings, error)
	WriteTemplateDeviceSettings(settings models.TemplateDeviceSettings) error
}

// templateStore wraps reading and writing different configurations locally
type templateStore struct {
	mu sync.Mutex
}

func (ts *templateStore) ReadTemplateURLs() (*models.TemplateURLs, error) {
	data, err := ts.readConfig(constants.PIDConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &models.TemplateURLs{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

func (ts *templateStore) WriteTemplateURLs(settings models.TemplateURLs) error {
	err := ts.writeConfig(constants.PIDConfigFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (ts *templateStore) ReadTemplateDeviceSettings() (*models.TemplateDeviceSettings, error) {
	data, err := ts.readConfig(constants.PIDConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &models.TemplateDeviceSettings{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

func (ts *templateStore) WriteTemplateDeviceSettings(settings models.TemplateDeviceSettings) error {
	err := ts.writeConfig(constants.PIDConfigFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func NewTemplateStore() TemplateStore {
	return &templateStore{}
}

func (ts *templateStore) ReadVINConfig() (*models.VINLoggerSettings, error) {
	data, err := ts.readConfig(constants.VINLoggerFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &models.VINLoggerSettings{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

func (ts *templateStore) WriteVINConfig(settings models.VINLoggerSettings) error {
	err := ts.writeConfig(constants.VINLoggerFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (ts *templateStore) ReadPIDsConfig() (*models.TemplatePIDs, error) {
	data, err := ts.readConfig(constants.PIDConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &models.TemplatePIDs{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

func (ts *templateStore) WritePIDsConfig(settings models.TemplatePIDs) error {
	err := ts.writeConfig(constants.PIDConfigFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (ts *templateStore) readConfig(filePath string) ([]byte, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}

	return data, nil
}

// WriteConfig writes the config file in json format to tmp folder, overwriting anything already existing
func (ts *templateStore) writeConfig(filePath string, settings interface{}) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
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
