package models

import "time"

type CanDumpData struct {
	CommonData
	Payload string `json:"payloadBase64,omitempty"`
}

// CommonData common properties we want to send with every data payload
type CommonData struct {
	RpiUptimeSecs  int     `json:"rpiUptimeSecs,omitempty"`
	BatteryVoltage float64 `json:"batteryVoltage,omitempty"`
	// Timestamp is in unix millis, when payload was sent
	Timestamp int64 `json:"timestamp"`
}

type DeviceStatusData struct {
	CommonData
	Signals []SignalData `json:"signals,omitempty"`
}

type SignalData struct {
	// Timestamp is in unix millis, when signal was queried
	Timestamp int64  `json:"timestamp"`
	Name      string `json:"name"`
	Value     any    `json:"value"`
}

type ErrorsData struct {
	CommonData
	Errors []string `json:"errors"`
}

type DeviceErrorsCloudEvent struct {
	CloudEventHeaders
	Data ErrorsData `json:"data"`
}

type FingerprintData struct {
	CommonData
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
