package loggers

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/DIMO-Network/shared/device"

	"github.com/pkg/errors"

	"github.com/DIMO-Network/edge-network/internal/models"
)

const (
	VINLoggerFile      = "/opt/autopi/vin-settings.json"
	PIDConfigFile      = "/opt/autopi/logger-pid-settings.json"
	TemplateURLsFile   = "/opt/autopi/template-urls.json"
	DeviceSettingsFile = "/opt/autopi/device-settings.json"
	VehicleInfoFile    = "/opt/autopi/vehicle-info.json"
	DBCFile            = "/opt/autopi/dbc-settings.dbc"
)

//go:generate mockgen -source template_store.go -destination mocks/template_store_mock.go
type TemplateStore interface {
	ReadVINConfig() (*models.VINLoggerSettings, error)
	WriteVINConfig(settings models.VINLoggerSettings) error

	ReadDBCFile() (*string, error)
	WriteDBCFile(dbcFile *string) error

	ReadPIDsConfig() (*models.TemplatePIDs, error)
	WritePIDsConfig(settings models.TemplatePIDs) error

	ReadTemplateURLs() (*device.ConfigResponse, error)
	WriteTemplateURLs(settings device.ConfigResponse) error

	ReadTemplateDeviceSettings() (*models.TemplateDeviceSettings, error)
	WriteTemplateDeviceSettings(settings models.TemplateDeviceSettings) error

	ReadVehicleInfo() (*models.VehicleInfo, error)
	WriteVehicleInfo(settings models.VehicleInfo) error
	DeleteAllSettings() error
}

// templateStore wraps reading and writing different configurations locally
type templateStore struct {
	mu sync.Mutex
}

func (ts *templateStore) DeleteAllSettings() error {
	var errs []error
	// Call each method and collect any errors
	errs = append(errs, ts.deleteConfig(VINLoggerFile))
	errs = append(errs, ts.deleteConfig(PIDConfigFile))
	errs = append(errs, ts.deleteConfig(DeviceSettingsFile))
	errs = append(errs, ts.deleteConfig(VehicleInfoFile))
	errs = append(errs, ts.deleteConfig(TemplateURLsFile))
	errs = append(errs, ts.deleteConfig(DBCFile))

	// Combine errors and print the result
	if combinedErr := combineErrors(errs); combinedErr != nil {
		return combinedErr
	}
	return nil
}

func (ts *templateStore) ReadDBCFile() (*string, error) {
	data, err := ts.readConfig(DBCFile)
	if err != nil {
		return nil, fmt.Errorf("error reading dbc file: %s", err)
	}

	dbc := string(data)

	return &dbc, nil
}

func (ts *templateStore) WriteDBCFile(dbcFile *string) error {
	if dbcFile == nil {
		return fmt.Errorf("dbcFile is required")
	}
	err := ts.writeConfig(DBCFile, *dbcFile)
	if err != nil {
		return fmt.Errorf("error writing dbc file: %s", err)
	}

	return nil
}

func (ts *templateStore) ReadTemplateURLs() (*device.ConfigResponse, error) {
	data, err := ts.readConfig(TemplateURLsFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &device.ConfigResponse{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	return ls, nil
}

func (ts *templateStore) WriteTemplateURLs(settings device.ConfigResponse) error {
	err := ts.writeConfig(TemplateURLsFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (ts *templateStore) ReadTemplateDeviceSettings() (*models.TemplateDeviceSettings, error) {
	data, err := ts.readConfig(DeviceSettingsFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &models.TemplateDeviceSettings{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	// set's the default value for min voltage
	if ls.MinVoltageOBDLoggers == 0 {
		ls.MinVoltageOBDLoggers = 13.3
	}

	return ls, nil
}

func (ts *templateStore) WriteTemplateDeviceSettings(settings models.TemplateDeviceSettings) error {
	err := ts.writeConfig(DeviceSettingsFile, settings)
	if err != nil {
		return err
	}

	return nil
}

// NewTemplateStore instantiates new instance of class used to read and write local configuration files
func NewTemplateStore() TemplateStore {
	return &templateStore{}
}

func (ts *templateStore) ReadVINConfig() (*models.VINLoggerSettings, error) {
	data, err := ts.readConfig(VINLoggerFile)
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
	err := ts.writeConfig(VINLoggerFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (ts *templateStore) ReadPIDsConfig() (*models.TemplatePIDs, error) {
	data, err := ts.readConfig(PIDConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	ls := &models.TemplatePIDs{}

	err = json.Unmarshal(data, ls)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall loggersettings: %s", err)
	}

	// check there is a resp header set
	for i, req := range ls.Requests {
		// set the default 7e8 if not set, we'll need a way to know if this vehicle is EFF
		if req.ResponseHeader == 0 {
			if strings.Contains(req.Protocol, "CAN29") {
				// extended frame
				ls.Requests[i].ResponseHeader = 417001744 // 0x18DAF110
			} else {
				// standard frame, set the default 7e8 if not set
				ls.Requests[i].ResponseHeader = 2024
			}
		}
	}

	return ls, nil
}

func (ts *templateStore) WritePIDsConfig(settings models.TemplatePIDs) error {
	err := ts.writeConfig(PIDConfigFile, settings)
	if err != nil {
		return err
	}

	return nil
}

func (ts *templateStore) ReadVehicleInfo() (*models.VehicleInfo, error) {
	data, err := ts.readConfig(VehicleInfoFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	vi := &models.VehicleInfo{}

	err = json.Unmarshal(data, vi)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall vehicleInfo: %s", err)
	}

	return vi, nil
}

func (ts *templateStore) WriteVehicleInfo(settings models.VehicleInfo) error {
	err := ts.writeConfig(VehicleInfoFile, settings)
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

	var data []byte

	if reflect.TypeOf(settings).Kind() == reflect.String {
		data = []byte(settings.(string))
	} else {
		data, err = json.Marshal(settings)
		if err != nil {
			return err
		}
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("error writing file: %s", err)
	}
	return nil
}

func (ts *templateStore) deleteConfig(filePath string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	// Open the file for writing (create if it doesn't exist)
	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("error deleting file: %s", err)
	}

	return nil
}

// CombineErrors combines multiple errors into a single error
func combineErrors(errorList []error) error {
	var errorMessages []string
	for _, err := range errorList {
		if err != nil {
			errorMessages = append(errorMessages, err.Error())
		}
	}
	if len(errorMessages) == 0 {
		return nil
	}
	return errors.New(strings.Join(errorMessages, "; "))
}
