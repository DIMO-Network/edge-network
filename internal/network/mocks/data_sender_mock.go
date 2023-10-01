// Code generated by MockGen. DO NOT EDIT.
// Source: data_sender.go

// Package mock_network is a generated GoMock package.
package mock_network

import (
	reflect "reflect"

	api "github.com/DIMO-Network/edge-network/internal/api"
	network "github.com/DIMO-Network/edge-network/internal/network"
	gomock "go.uber.org/mock/gomock"
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

// SendDeviceStatusData mocks base method.
func (m *MockDataSender) SendDeviceStatusData(data network.DeviceStatusData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendDeviceStatusData", data)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendDeviceStatusData indicates an expected call of SendDeviceStatusData.
func (mr *MockDataSenderMockRecorder) SendDeviceStatusData(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendDeviceStatusData", reflect.TypeOf((*MockDataSender)(nil).SendDeviceStatusData), data)
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

// SendErrorsData mocks base method.
func (m *MockDataSender) SendErrorsData(data network.ErrorsData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SendErrorsData", data)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendErrorsData indicates an expected call of SendErrorsData.
func (mr *MockDataSenderMockRecorder) SendErrorsData(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendErrorsData", reflect.TypeOf((*MockDataSender)(nil).SendErrorsData), data)
}

// SendFingerprintData mocks base method.
func (m *MockDataSender) SendFingerprintData(data network.FingerprintData) error {
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
