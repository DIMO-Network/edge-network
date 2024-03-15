package models

import (
	"github.com/DIMO-Network/edge-network/internal/api"
	"time"
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
	Device   Device    `json:"device,omitempty"`
	Vehicle  Vehicle   `json:"vehicle,omitempty"`
	Location *Location `json:"location,omitempty"`
	Network  *Network  `json:"network,omitempty"`
}

type Device struct {
	RpiUptimeSecs  int     `json:"rpiUptimeSecs,omitempty"`
	BatteryVoltage float64 `json:"batteryVoltage,omitempty"`
}

type Network struct {
	WiFi WiFi `json:"wifi,omitempty"`
	// consider to not import api here
	QMICellInfoResponse api.QMICellInfoResponse `json:"cell,omitempty"`
}

type Vehicle struct {
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
}

type SignalData struct {
	// Timestamp is in unix millis, when signal was queried
	Timestamp int64  `json:"timestamp"`
	Name      string `json:"name"`
	Value     any    `json:"value"`
}

type ErrorsData struct {
	CommonData
	Device Device   `json:"device,omitempty"`
	Errors []string `json:"errors"`
}

type DeviceErrorsCloudEvent struct {
	CloudEventHeaders
	Data ErrorsData `json:"data"`
}

type FingerprintData struct {
	CommonData
	Device          Device  `json:"device,omitempty"`
	Vin             string  `json:"vin"`
	Protocol        string  `json:"protocol"`
	Odometer        float64 `json:"odometer,omitempty"`
	SoftwareVersion string  `json:"softwareVersion"`
}

// CloudEventHeaders contains the fields common to all CloudEvent messages. https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md
type CloudEventHeaders struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	SpecVersion string    `json:"specversion"`
	Subject     string    `json:"subject"`
	Time        time.Time `json:"time"`
	Type        string    `json:"type"`
	// Signature is an extension https://github.com/cloudevents/spec/blob/main/cloudevents/documented-extensions.md
	Signature string `json:"signature"`
}

type CanDumpCloudEvent struct {
	CloudEventHeaders
	Data CanDumpData `json:"data"`
}

type DeviceFingerprintCloudEvent struct {
	CloudEventHeaders
	Data FingerprintData `json:"data"`
}

type DeviceDataStatusCloudEvent struct {
	CloudEventHeaders
	Data DeviceStatusData `json:"data"`
}
