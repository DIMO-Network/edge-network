// Code generated by MockGen. DO NOT EDIT.
// Source: vehicle_signal_decoding_service.go

// Package mock_gateways is a generated GoMock package.
package mock_gateways

import (
	reflect "reflect"

	gateways "github.com/DIMO-Network/edge-network/internal/gateways"
	gomock "github.com/golang/mock/gomock"
)

// MockVehicleSignalDecodingAPIService is a mock of VehicleSignalDecodingAPIService interface.
type MockVehicleSignalDecodingAPIService struct {
	ctrl     *gomock.Controller
	recorder *MockVehicleSignalDecodingAPIServiceMockRecorder
}

// MockVehicleSignalDecodingAPIServiceMockRecorder is the mock recorder for MockVehicleSignalDecodingAPIService.
type MockVehicleSignalDecodingAPIServiceMockRecorder struct {
	mock *MockVehicleSignalDecodingAPIService
}

// NewMockVehicleSignalDecodingAPIService creates a new mock instance.
func NewMockVehicleSignalDecodingAPIService(ctrl *gomock.Controller) *MockVehicleSignalDecodingAPIService {
	mock := &MockVehicleSignalDecodingAPIService{ctrl: ctrl}
	mock.recorder = &MockVehicleSignalDecodingAPIServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVehicleSignalDecodingAPIService) EXPECT() *MockVehicleSignalDecodingAPIServiceMockRecorder {
	return m.recorder
}

// GetPIDsTemplateByVIN mocks base method.
func (m *MockVehicleSignalDecodingAPIService) GetPIDsTemplateByVIN(vin string) (*gateways.PIDConfigResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPIDsTemplateByVIN", vin)
	ret0, _ := ret[0].(*gateways.PIDConfigResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPIDsTemplateByVIN indicates an expected call of GetPIDsTemplateByVIN.
func (mr *MockVehicleSignalDecodingAPIServiceMockRecorder) GetPIDsTemplateByVIN(vin interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPIDsTemplateByVIN", reflect.TypeOf((*MockVehicleSignalDecodingAPIService)(nil).GetPIDsTemplateByVIN), vin)
}
