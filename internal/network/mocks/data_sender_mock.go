// Code generated by MockGen. DO NOT EDIT.
// Source: data_sender.go

// Package mock_network is a generated GoMock package.
package mock_network

import (
	"go.uber.org/mock/gomock"
	reflect "reflect"

	api "github.com/DIMO-Network/edge-network/internal/api"
	models "github.com/DIMO-Network/edge-network/internal/models"
)

// MockDataSender is a mock of DataSender interface.
type MockDataSender struct {
	ctrl     *gomock.Controller
	recorder *MockDataSenderMockRecorder
}

// MockDataSenderMockRecorder is the mock recorder for MockDataSender.
type MockDataSenderMockRecorder struct {
	mock *MockDataSender
}

// NewMockDataSender creates a new mock instance.
func NewMockDataSender(ctrl *gomock.Controller) *MockDataSender {
	mock := &MockDataSender{ctrl: ctrl}
	mock.recorder = &MockDataSenderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDataSender) EXPECT() *MockDataSenderMockRecorder {
	return m.recorder
}

// SendCanDumpData mocks base method.
func (m *MockDataSender) SendCanDumpData(data models.CanDumpData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendCanDumpData", data)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendCanDumpData indicates an expected call of SendCanDumpData.
func (mr *MockDataSenderMockRecorder) SendCanDumpData(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendCanDumpData", reflect.TypeOf((*MockDataSender)(nil).SendCanDumpData), data)
}

// SendDeviceNetworkData mocks base method.
func (m *MockDataSender) SendDeviceNetworkData(data models.DeviceNetworkData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendDeviceNetworkData", data)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendDeviceNetworkData indicates an expected call of SendDeviceNetworkData.
func (mr *MockDataSenderMockRecorder) SendDeviceNetworkData(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendDeviceNetworkData", reflect.TypeOf((*MockDataSender)(nil).SendDeviceNetworkData), data)
}

// SendDeviceStatusData mocks base method.
func (m *MockDataSender) SendDeviceStatusData(data any, tokenID uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendDeviceStatusData", data, tokenID)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendDeviceStatusData indicates an expected call of SendDeviceStatusData.
func (mr *MockDataSenderMockRecorder) SendDeviceStatusData(data, tokenID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendDeviceStatusData", reflect.TypeOf((*MockDataSender)(nil).SendDeviceStatusData), data, tokenID)
}

// SendErrorPayload mocks base method.
func (m *MockDataSender) SendErrorPayload(err error, powerStatus *api.PowerStatusResponse) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendErrorPayload", err, powerStatus)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendErrorPayload indicates an expected call of SendErrorPayload.
func (mr *MockDataSenderMockRecorder) SendErrorPayload(err, powerStatus interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendErrorPayload", reflect.TypeOf((*MockDataSender)(nil).SendErrorPayload), err, powerStatus)
}

// SendFingerprintData mocks base method.
func (m *MockDataSender) SendFingerprintData(data models.FingerprintData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendFingerprintData", data)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendFingerprintData indicates an expected call of SendFingerprintData.
func (mr *MockDataSenderMockRecorder) SendFingerprintData(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendFingerprintData", reflect.TypeOf((*MockDataSender)(nil).SendFingerprintData), data)
}

// SendLogsData mocks base method.
func (m *MockDataSender) SendLogsData(data models.ErrorsData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendLogsData", data)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendLogsData indicates an expected call of SendLogsData.
func (mr *MockDataSenderMockRecorder) SendLogsData(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendLogsData", reflect.TypeOf((*MockDataSender)(nil).SendLogsData), data)
}
