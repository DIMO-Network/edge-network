package models

import (
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/shared"
)

type CanDumpData struct {
	CommonData
	Payload string `json:"payloadBase64,omitempty"`
}

// CommonData common properties we want to send with every data payload
type CommonData struct {
	// Timestamp is in unix millis, when payload was sent
	Timestamp int64 `json:"timestamp"`
}

type DeviceStatusData struct {
	CommonData
	Device  Device  `json:"device,omitempty"`
	Vehicle Vehicle `json:"vehicle,omitempty"`
}

// DeviceNetworkData is used to submit to the cellular coverage firehose. Should have: timestamp, cell.details, latitude, longitude, altitude, nsat, hdop
type DeviceNetworkData struct {
	CommonData
	Location
	Cell CellInfo `json:"cell,omitempty"`
}

type CompressedPayload struct {
	Payload string `json:"compressedPayload,omitempty"`
}

type Device struct {
	RpiUptimeSecs   int     `json:"rpiUptimeSecs,omitempty"`
	BatteryVoltage  float64 `json:"batteryVoltage,omitempty"`
	SoftwareVersion string  `json:"softwareVersion,omitempty"`
	HardwareVersion string  `json:"hwVersion,omitempty"`
	IMEI            string  `json:"imei,omitempty"`
	UnitID          string  `json:"serial,omitempty"`
}

type Vehicle struct {
	Make    string       `json:"make"`
	Model   string       `json:"model"`
	Year    int          `json:"year"`
	Signals []SignalData `json:"signals,omitempty"`
}

type WiFi struct {
	WPAState string `json:"wpa_state,omitempty"`
	SSID     string `json:"ssid,omitempty"`
}

type Location struct {
	Hdop      float64 `json:"hdop,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Nsat      int64   `json:"nsat,omitempty"`
	Altitude  float64 `json:"altitude,omitempty"`
}

type SignalData struct {
	// Timestamp is in unix millis, when signal was queried
	Timestamp int64  `json:"timestamp"`
	Name      string `json:"name"`
	Value     any    `json:"value"`
}

type ErrorsData struct {
	CommonData
	Device    Device `json:"device,omitempty"`
	TokenID   uint64 `json:"vehicleTokenId"`
	ModelSlug string `json:"modelSlug"`
	// deprecated
	Errors  []string `json:"errors"`
	Message string   `json:"message"`
	Level   string   `json:"level"`
}

type FingerprintData struct {
	CommonData
	Device          Device  `json:"device,omitempty"`
	Vin             string  `json:"vin"`
	Protocol        string  `json:"protocol"`
	Odometer        float64 `json:"odometer,omitempty"`
	SoftwareVersion string  `json:"softwareVersion"`
}

type CellInfo struct {
	Details api.IntrafrequencyLteInfo `json:"details,omitempty"`
	IP      string                    `json:"ip,omitempty"`
}

type VehicleInfo struct {
	TokenID           uint64            `json:"tokenId"`
	VehicleDefinition VehicleDefinition `json:"definition"`
}

type VehicleDefinition struct {
	Make  string `json:"make"`
	Model string `json:"model"`
	Year  int    `json:"year"`
}

type GraphQLRequest struct {
	Query string `json:"query"`
}

type DeviceDataStatusCloudEvent[A any] struct {
	shared.CloudEvent[A]
	TokenID uint64 `json:"vehicleTokenId"`
}

// CAN Dump frame project

type SignalCanFrameDump struct {
	Timestamp int64 `json:"timestamp"`
	// the Signal Name
	Name          string `json:"name"`
	HexValue      string `json:"hexValue"`
	Pid           uint32 `json:"pid"`
	Error         string `json:"error"`
	PythonFormula string `json:"pythonFormula"`
}
