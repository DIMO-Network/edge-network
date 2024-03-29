package network

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/segmentio/ksuid"
	"github.com/tidwall/gjson"

	"github.com/DIMO-Network/edge-network/commands"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/sjson"
)

// thought: should we have another topic for errors? ie. signals we could not get

const broker = "tcp://localhost:1883"

//go:generate mockgen -source data_sender.go -destination mocks/data_sender_mock.go
type DataSender interface {
	SendErrorPayload(err error, powerStatus *api.PowerStatusResponse) error
	SendErrorsData(data ErrorsData) error
	SendFingerprintData(data FingerprintData) error
	SendCanDumpData(data CanDumpData) error
}

type dataSender struct {
	client  mqtt.Client
	unitID  uuid.UUID
	ethAddr common.Address
	topic   string
}

// NewDataSender instantiates new data sender, does not create a connection to broker
func NewDataSender(unitID uuid.UUID, addr common.Address, topic string) DataSender {
	// Setup mqtt connection. Does not connect
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	return &dataSender{
		client:  client,
		unitID:  unitID,
		ethAddr: addr,
		topic:   topic,
	}
}

func (ds *dataSender) SendFingerprintData(data FingerprintData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}
	ceh := newCloudEventHeaders(ds.ethAddr, "aftermarket/device/fingerprint", "1.0", "zone.dimo.aftermarket.device.fingerprint")
	ce := DeviceFingerprintCloudEvent{
		CloudEventHeaders: ceh,
		Data:              data,
	}
	payload, err := json.Marshal(ce)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	err = ds.sendPayload(payload)
	if err != nil {
		return err
	}
	return nil
}

func (ds *dataSender) SendCanDumpData(data CanDumpData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}
	ceh := newCloudEventHeaders(ds.ethAddr, "aftermarket/device/canbus/dump", "1.0", "zone.dimo.aftermarket.canbus.dump")
	ce := CanDumpCloudEvent{
		CloudEventHeaders: ceh,
		Data:              data,
	}
	println("Sending can dump data: (payload)")
	payload, err := json.Marshal(ce)
	println(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	err = ds.sendPayload(payload)
	if err != nil {
		return err
	}
	return nil
}

func (ds *dataSender) SendErrorsData(data ErrorsData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}
	ceh := newCloudEventHeaders(ds.ethAddr, "aftermarket/device/fingerprint", "1.0", "zone.dimo.aftermarket.device.fingerprint")
	ce := DeviceErrorsCloudEvent{
		CloudEventHeaders: ceh,
		Data:              data,
	}
	payload, err := json.Marshal(ce)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	err = ds.sendPayload(payload)
	if err != nil {
		return err
	}
	return nil
}

// SendPayload connects to broker and sends a filled in status update via mqtt to broker address, should already be in json format
func (ds *dataSender) sendPayload(payload []byte) error {
	// todo: determine if we want to be connecting and disconnecting from mqtt broker for every status update we send (when start sending more periodic data besides VIN)
	if !gjson.GetBytes(payload, "subject").Exists() {
		return fmt.Errorf("payload did not have expected subject cloud event property")
	}

	log.Infof("sending payload:\n")
	log.Infof("%s", string(payload))

	// Connect to the MQTT broker
	if token := ds.client.Connect(); token.Wait() && token.Error() != nil {
		return errors.Wrap(token.Error(), "failed to connect to mqtt broker")
	}

	// Wait for the connection to be established
	for !ds.client.IsConnected() {
		time.Sleep(100 * time.Millisecond)
		// todo timeout?
	}
	// Disconnect from the MQTT broker
	defer ds.client.Disconnect(250)

	// signature for the payload
	payload, err := signPayload(payload, ds.unitID)
	if err != nil {
		return err
	}
	// Publish the MQTT message
	token := ds.client.Publish(ds.topic, 0, false, string(payload))
	token.Wait() // just waits up until message goes through

	// Check if the message was successfully published
	if token.Error() != nil {
		return errors.Wrap(err, "Failed to publish MQTT message")
	}

	return nil
}

func signPayload(payload []byte, unitID uuid.UUID) ([]byte, error) {
	dataResult := gjson.GetBytes(payload, "data")
	if !dataResult.Exists() {
		return nil, fmt.Errorf("no data json path found to sign")
	}

	keccak256Hash := crypto.Keccak256Hash([]byte(dataResult.Raw))
	sig, err := commands.SignHash(unitID, keccak256Hash.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign the status update")
	}
	signature := "0x" + hex.EncodeToString(sig)
	// note the path should match the CloudEventHeaders signature name
	payload, err = sjson.SetBytes(payload, "signature", signature)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add signature to status update")
	}
	return payload, nil
}

func (ds *dataSender) SendErrorPayload(err error, powerStatus *api.PowerStatusResponse) error {
	data := ErrorsData{}
	if powerStatus != nil {
		data.BatteryVoltage = powerStatus.Spm.Battery.Voltage
		data.RpiUptimeSecs = powerStatus.Rpi.Uptime.Seconds
	}
	data.Errors = append(data.Errors, err.Error())

	return ds.SendErrorsData(data)
}

func newCloudEventHeaders(ethAddress common.Address, source string, specVersion string, eventType string) CloudEventHeaders {
	ce := CloudEventHeaders{
		ID:          ksuid.New().String(),
		Source:      source,
		SpecVersion: specVersion,
		Subject:     ethAddress.Hex(),
		Time:        time.Now().UTC(),
		Type:        eventType,
	}
	return ce
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

type FingerprintData struct {
	CommonData
	Vin      string  `json:"vin"`
	Protocol string  `json:"protocol"`
	Odometer float64 `json:"odometer,omitempty"`
}

type CanDumpData struct {
	CommonData
	Payload string `json:"payloadBase64,omitempty"`
}

type DeviceErrorsCloudEvent struct {
	CloudEventHeaders
	Data ErrorsData `json:"data"`
}

type ErrorsData struct {
	CommonData
	Errors []string `json:"errors"`
}

// CommonData common properties we want to send with every data payload
type CommonData struct {
	RpiUptimeSecs  int     `json:"rpiUptimeSecs,omitempty"`
	BatteryVoltage float64 `json:"batteryVoltage,omitempty"`
	Timestamp      int64   `json:"timestamp"`
}
