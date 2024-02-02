// Code generated by MockGen. DO NOT EDIT.
// Source: vin_logger.go
//
// Generated by this command:
//
//	mockgen -source vin_logger.go -destination mocks/vin_logger_mock.go
//
// Package mock_loggers is a generated GoMock package.
package mock_loggers

import (
	reflect "reflect"

	loggers "github.com/DIMO-Network/edge-network/internal/loggers"
	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

// MockVINLogger is a mock of VINLogger interface.
type MockVINLogger struct {
	ctrl     *gomock.Controller
	recorder *MockVINLoggerMockRecorder
}

// MockVINLoggerMockRecorder is the mock recorder for MockVINLogger.
type MockVINLoggerMockRecorder struct {
	mock *MockVINLogger
}

// NewMockVINLogger creates a new mock instance.
func NewMockVINLogger(ctrl *gomock.Controller) *MockVINLogger {
	mock := &MockVINLogger{ctrl: ctrl}
	mock.recorder = &MockVINLoggerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVINLogger) EXPECT() *MockVINLoggerMockRecorder {
	return m.recorder
}

// GetVIN mocks base method.
func (m *MockVINLogger) GetVIN(unitID uuid.UUID, queryName *string) (*loggers.VINResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVIN", unitID, queryName)
	ret0, _ := ret[0].(*loggers.VINResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetVIN indicates an expected call of GetVIN.
func (mr *MockVINLoggerMockRecorder) GetVIN(unitID, queryName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVIN", reflect.TypeOf((*MockVINLogger)(nil).GetVIN), unitID, queryName)
}
