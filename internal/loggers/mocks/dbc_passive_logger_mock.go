// Code generated by MockGen. DO NOT EDIT.
// Source: dbc_passive_logger.go
//
// Generated by this command:
//
//	mockgen -source dbc_passive_logger.go -destination mocks/dbc_passive_logger_mock.go
//
// Package mock_loggers is a generated GoMock package.
package mock_loggers

import (
	reflect "reflect"

	models "github.com/DIMO-Network/edge-network/internal/models"
	gomock "go.uber.org/mock/gomock"
)

// MockDBCPassiveLogger is a mock of DBCPassiveLogger interface.
type MockDBCPassiveLogger struct {
	ctrl     *gomock.Controller
	recorder *MockDBCPassiveLoggerMockRecorder
}

// MockDBCPassiveLoggerMockRecorder is the mock recorder for MockDBCPassiveLogger.
type MockDBCPassiveLoggerMockRecorder struct {
	mock *MockDBCPassiveLogger
}

// NewMockDBCPassiveLogger creates a new mock instance.
func NewMockDBCPassiveLogger(ctrl *gomock.Controller) *MockDBCPassiveLogger {
	mock := &MockDBCPassiveLogger{ctrl: ctrl}
	mock.recorder = &MockDBCPassiveLoggerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDBCPassiveLogger) EXPECT() *MockDBCPassiveLoggerMockRecorder {
	return m.recorder
}

// HasDBCFile mocks base method.
func (m *MockDBCPassiveLogger) HasDBCFile() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasDBCFile")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasDBCFile indicates an expected call of HasDBCFile.
func (mr *MockDBCPassiveLoggerMockRecorder) HasDBCFile() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasDBCFile", reflect.TypeOf((*MockDBCPassiveLogger)(nil).HasDBCFile))
}

// SendCANQuery mocks base method.
func (m *MockDBCPassiveLogger) SendCANQuery(header, mode, pid uint32) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendCANQuery", header, mode, pid)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendCANQuery indicates an expected call of SendCANQuery.
func (mr *MockDBCPassiveLoggerMockRecorder) SendCANQuery(header, mode, pid any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendCANQuery", reflect.TypeOf((*MockDBCPassiveLogger)(nil).SendCANQuery), header, mode, pid)
}

// StartScanning mocks base method.
func (m *MockDBCPassiveLogger) StartScanning(ch chan<- models.SignalData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartScanning", ch)
	ret0, _ := ret[0].(error)
	return ret0
}

// StartScanning indicates an expected call of StartScanning.
func (mr *MockDBCPassiveLoggerMockRecorder) StartScanning(ch any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartScanning", reflect.TypeOf((*MockDBCPassiveLogger)(nil).StartScanning), ch)
}

// StopScanning mocks base method.
func (m *MockDBCPassiveLogger) StopScanning() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StopScanning")
	ret0, _ := ret[0].(error)
	return ret0
}

// StopScanning indicates an expected call of StopScanning.
func (mr *MockDBCPassiveLoggerMockRecorder) StopScanning() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopScanning", reflect.TypeOf((*MockDBCPassiveLogger)(nil).StopScanning))
}
