package network

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/segmentio/ksuid"
	"github.com/tidwall/gjson"

	"github.com/DIMO-Network/edge-network/commands"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
)

// thought: should we have a different topic for errors? eg. signals we could not get, failed fingerprinting
const fingerprintTopic = "fingerprint"
const broker = "tcp://localhost:1883" // local mqtt broker address

//go:generate mockgen -source data_sender.go -destination mocks/data_sender_mock.go
type DataSender interface {
	SendErrorPayload(err error, powerStatus *api.PowerStatusResponse) error
	SendErrorsData(data ErrorsData) error
	// SendFingerprintData sends VIN and protocol over mqtt to corresponding topic, could add anything else to help identify vehicle
	SendFingerprintData(data FingerprintData) error
	SendCanDumpData(data CanDumpData) error
	// SendDeviceStatusData sends queried vehicle data over mqtt, per configuration from vehicle-signal-decoding api
	SendDeviceStatusData(data DeviceStatusData) error
}

type dataSender struct {
	client  mqtt.Client
	unitID  uuid.UUID
	ethAddr common.Address
	logger  zerolog.Logger
	topic   string
}

// NewDataSender instantiates new data sender, does not create a connection to broker
func NewDataSender(unitID uuid.UUID, addr common.Address, logger zerolog.Logger, topic string) DataSender {
	// Setup mqtt connection. Does not connect
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	return &dataSender{
		client:  client,
		unitID:  unitID,
		ethAddr: addr,
		logger:  logger,
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
		ds.logger.Error().Err(err).Msg("failed to marshall cloudevent")
		return errors.Wrap(err, "failed to marshall cloudevent")
	}
	ds.logger.Debug().Msgf("sending fingerprint payload: %s", string(payload))

	err = ds.sendPayload(fingerprintTopic, payload)
	if err != nil {
		ds.logger.Error().Err(err).Msg("failed send payload")
		return err
	}
	return nil
}

func (ds *dataSender) SendDeviceStatusData(data DeviceStatusData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}
	// todo validate what source should be here
	ceh := newCloudEventHeaders(ds.ethAddr, "aftermarket/device/status", "1.0", "com.dimo.device.status")
	ce := DeviceDataStatusCloudEvent{
		CloudEventHeaders: ceh,
		Data:              data,
	}
	payload, err := json.Marshal(ce)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}
	ds.logger.Debug().Msgf("sending status payload: %s", string(payload))

	//err = ds.sendPayload(payload) what topic to use?
	//if err != nil {
	//	return err
	//}
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
	// i'm not sure this is correct, the datasender should hold the different topics in a const above and just use them here - not for other modules to decide.
	err = ds.sendPayload(ds.topic, payload)
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

	err = ds.sendPayload(fingerprintTopic, payload) // i think this is a bit funky, like errors should go in their own topic, we want to build an observable system
	if err != nil {
		return err
	}
	return nil
}

// SendPayload connects to broker and sends a filled in status update via mqtt to broker address, should already be in json format
func (ds *dataSender) sendPayload(topic string, payload []byte) error {
	// todo I think we want to chage topic to come from parameter not from datasender value
	// todo: determine if we want to be connecting and disconnecting from mqtt broker for every status update we send (when start sending more periodic data besides VIN)
	if !gjson.GetBytes(payload, "subject").Exists() {
		return fmt.Errorf("payload did not have expected subject cloud event property")
	}

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

type DeviceDataStatusCloudEvent struct {
	CloudEventHeaders
	Data DeviceStatusData `json:"data"`
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

type DeviceStatusData struct {
	CommonData
	Signals []SignalData `json:"signals,omitempty"`
}

type SignalData struct {
	// Timestamp is in unix millis, when signal was queried
	Timestamp int64  `json:"timestamp"`
	Name      string `json:"name"`
	Value     string `json:"value"`
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
	// Timestamp is in unix millis, when payload was sent
	Timestamp int64 `json:"timestamp"`
}
