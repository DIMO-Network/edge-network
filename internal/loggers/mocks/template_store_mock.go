// Code generated by MockGen. DO NOT EDIT.
// Source: template_store.go
//
// Generated by this command:
//
//	mockgen -source template_store.go -destination mocks/template_store_mock.go
//
// Package mock_loggers is a generated GoMock package.
package mock_loggers

import (
	reflect "reflect"

	models "github.com/DIMO-Network/edge-network/internal/models"
	device "github.com/DIMO-Network/shared/device"
	gomock "go.uber.org/mock/gomock"
)

// MockSettingsStore is a mock of SettingsStore interface.
type MockSettingsStore struct {
	ctrl     *gomock.Controller
	recorder *MockSettingsStoreMockRecorder
}

// MockSettingsStoreMockRecorder is the mock recorder for MockSettingsStore.
type MockSettingsStoreMockRecorder struct {
	mock *MockSettingsStore
}

// NewMockSettingsStore creates a new mock instance.
func NewMockSettingsStore(ctrl *gomock.Controller) *MockSettingsStore {
	mock := &MockSettingsStore{ctrl: ctrl}
	mock.recorder = &MockSettingsStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSettingsStore) EXPECT() *MockSettingsStoreMockRecorder {
	return m.recorder
}

// DeleteAllSettings mocks base method.
func (m *MockSettingsStore) DeleteAllSettings() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAllSettings")
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAllSettings indicates an expected call of DeleteAllSettings.
func (mr *MockSettingsStoreMockRecorder) DeleteAllSettings() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAllSettings", reflect.TypeOf((*MockSettingsStore)(nil).DeleteAllSettings))
}

// ReadCANDumpInfo mocks base method.
func (m *MockSettingsStore) ReadCANDumpInfo() (*models.CANDumpInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadCANDumpInfo")
	ret0, _ := ret[0].(*models.CANDumpInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadCANDumpInfo indicates an expected call of ReadCANDumpInfo.
func (mr *MockSettingsStoreMockRecorder) ReadCANDumpInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadCANDumpInfo", reflect.TypeOf((*MockSettingsStore)(nil).ReadCANDumpInfo))
}

// ReadDBCFile mocks base method.
func (m *MockSettingsStore) ReadDBCFile() (*string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDBCFile")
	ret0, _ := ret[0].(*string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDBCFile indicates an expected call of ReadDBCFile.
func (mr *MockSettingsStoreMockRecorder) ReadDBCFile() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDBCFile", reflect.TypeOf((*MockSettingsStore)(nil).ReadDBCFile))
}

// ReadPIDsConfig mocks base method.
func (m *MockSettingsStore) ReadPIDsConfig() (*models.TemplatePIDs, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadPIDsConfig")
	ret0, _ := ret[0].(*models.TemplatePIDs)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadPIDsConfig indicates an expected call of ReadPIDsConfig.
func (mr *MockSettingsStoreMockRecorder) ReadPIDsConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadPIDsConfig", reflect.TypeOf((*MockSettingsStore)(nil).ReadPIDsConfig))
}

// ReadTemplateDeviceSettings mocks base method.
func (m *MockSettingsStore) ReadTemplateDeviceSettings() (*models.TemplateDeviceSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadTemplateDeviceSettings")
	ret0, _ := ret[0].(*models.TemplateDeviceSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadTemplateDeviceSettings indicates an expected call of ReadTemplateDeviceSettings.
func (mr *MockSettingsStoreMockRecorder) ReadTemplateDeviceSettings() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadTemplateDeviceSettings", reflect.TypeOf((*MockSettingsStore)(nil).ReadTemplateDeviceSettings))
}

// ReadTemplateURLs mocks base method.
func (m *MockSettingsStore) ReadTemplateURLs() (*device.ConfigResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadTemplateURLs")
	ret0, _ := ret[0].(*device.ConfigResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadTemplateURLs indicates an expected call of ReadTemplateURLs.
func (mr *MockSettingsStoreMockRecorder) ReadTemplateURLs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadTemplateURLs", reflect.TypeOf((*MockSettingsStore)(nil).ReadTemplateURLs))
}

// ReadVINConfig mocks base method.
func (m *MockSettingsStore) ReadVINConfig() (*models.VINLoggerSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadVINConfig")
	ret0, _ := ret[0].(*models.VINLoggerSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadVINConfig indicates an expected call of ReadVINConfig.
func (mr *MockSettingsStoreMockRecorder) ReadVINConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadVINConfig", reflect.TypeOf((*MockSettingsStore)(nil).ReadVINConfig))
}

// ReadVehicleInfo mocks base method.
func (m *MockSettingsStore) ReadVehicleInfo() (*models.VehicleInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadVehicleInfo")
	ret0, _ := ret[0].(*models.VehicleInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadVehicleInfo indicates an expected call of ReadVehicleInfo.
func (mr *MockSettingsStoreMockRecorder) ReadVehicleInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadVehicleInfo", reflect.TypeOf((*MockSettingsStore)(nil).ReadVehicleInfo))
}

// WriteCANDumpInfo mocks base method.
func (m *MockSettingsStore) WriteCANDumpInfo() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteCANDumpInfo")
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteCANDumpInfo indicates an expected call of WriteCANDumpInfo.
func (mr *MockSettingsStoreMockRecorder) WriteCANDumpInfo() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteCANDumpInfo", reflect.TypeOf((*MockSettingsStore)(nil).WriteCANDumpInfo))
}

// WriteDBCFile mocks base method.
func (m *MockSettingsStore) WriteDBCFile(dbcFile *string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteDBCFile", dbcFile)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteDBCFile indicates an expected call of WriteDBCFile.
func (mr *MockSettingsStoreMockRecorder) WriteDBCFile(dbcFile any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteDBCFile", reflect.TypeOf((*MockSettingsStore)(nil).WriteDBCFile), dbcFile)
}

// WritePIDsConfig mocks base method.
func (m *MockSettingsStore) WritePIDsConfig(settings models.TemplatePIDs) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WritePIDsConfig", settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// WritePIDsConfig indicates an expected call of WritePIDsConfig.
func (mr *MockSettingsStoreMockRecorder) WritePIDsConfig(settings any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WritePIDsConfig", reflect.TypeOf((*MockSettingsStore)(nil).WritePIDsConfig), settings)
}

// WriteTemplateDeviceSettings mocks base method.
func (m *MockSettingsStore) WriteTemplateDeviceSettings(settings models.TemplateDeviceSettings) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteTemplateDeviceSettings", settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteTemplateDeviceSettings indicates an expected call of WriteTemplateDeviceSettings.
func (mr *MockSettingsStoreMockRecorder) WriteTemplateDeviceSettings(settings any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteTemplateDeviceSettings", reflect.TypeOf((*MockSettingsStore)(nil).WriteTemplateDeviceSettings), settings)
}

// WriteTemplateURLs mocks base method.
func (m *MockSettingsStore) WriteTemplateURLs(settings device.ConfigResponse) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteTemplateURLs", settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteTemplateURLs indicates an expected call of WriteTemplateURLs.
func (mr *MockSettingsStoreMockRecorder) WriteTemplateURLs(settings any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteTemplateURLs", reflect.TypeOf((*MockSettingsStore)(nil).WriteTemplateURLs), settings)
}

// WriteVINConfig mocks base method.
func (m *MockSettingsStore) WriteVINConfig(settings models.VINLoggerSettings) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteVINConfig", settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteVINConfig indicates an expected call of WriteVINConfig.
func (mr *MockSettingsStoreMockRecorder) WriteVINConfig(settings any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteVINConfig", reflect.TypeOf((*MockSettingsStore)(nil).WriteVINConfig), settings)
}

// WriteVehicleInfo mocks base method.
func (m *MockSettingsStore) WriteVehicleInfo(settings models.VehicleInfo) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteVehicleInfo", settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteVehicleInfo indicates an expected call of WriteVehicleInfo.
func (mr *MockSettingsStoreMockRecorder) WriteVehicleInfo(settings any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteVehicleInfo", reflect.TypeOf((*MockSettingsStore)(nil).WriteVehicleInfo), settings)
}
