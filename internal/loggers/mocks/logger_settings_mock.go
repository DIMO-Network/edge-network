// Code generated by MockGen. DO NOT EDIT.
// Source: logger_settings.go

// Package mock_loggers is a generated GoMock package.
package mock_loggers

import (
	reflect "reflect"

	loggers "github.com/DIMO-Network/edge-network/internal/loggers"
	gomock "github.com/golang/mock/gomock"
)

// MockLoggerSettingsService is a mock of LoggerSettingsService interface.
type MockLoggerSettingsService struct {
	ctrl     *gomock.Controller
	recorder *MockLoggerSettingsServiceMockRecorder
}

// MockLoggerSettingsServiceMockRecorder is the mock recorder for MockLoggerSettingsService.
type MockLoggerSettingsServiceMockRecorder struct {
	mock *MockLoggerSettingsService
}

// NewMockLoggerSettingsService creates a new mock instance.
func NewMockLoggerSettingsService(ctrl *gomock.Controller) *MockLoggerSettingsService {
	mock := &MockLoggerSettingsService{ctrl: ctrl}
	mock.recorder = &MockLoggerSettingsServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLoggerSettingsService) EXPECT() *MockLoggerSettingsServiceMockRecorder {
	return m.recorder
}

// ReadConfig mocks base method.
func (m *MockLoggerSettingsService) ReadConfig() (*loggers.LoggerSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadConfig")
	ret0, _ := ret[0].(*loggers.LoggerSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadConfig indicates an expected call of ReadConfig.
func (mr *MockLoggerSettingsServiceMockRecorder) ReadConfig() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadConfig", reflect.TypeOf((*MockLoggerSettingsService)(nil).ReadConfig))
}

// WriteConfig mocks base method.
func (m *MockLoggerSettingsService) WriteConfig(settings loggers.LoggerSettings) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteConfig", settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteConfig indicates an expected call of WriteConfig.
func (mr *MockLoggerSettingsServiceMockRecorder) WriteConfig(settings interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteConfig", reflect.TypeOf((*MockLoggerSettingsService)(nil).WriteConfig), settings)
}
