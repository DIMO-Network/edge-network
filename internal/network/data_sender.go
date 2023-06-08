package network

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/segmentio/ksuid"
	"github.com/tidwall/gjson"
	"time"

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
const topic = "fingerprint"
const broker = "tcp://localhost:1883"

//go:generate mockgen -source data_sender.go -destination mocks/data_sender_mock.go
type DataSender interface {
	SendErrorPayload(err error, powerStatus *api.PowerStatusResponse) error
	SendErrorsData(data ErrorsData) error
	SendFingerprintData(data FingerprintData) error
}

type dataSender struct {
	client  mqtt.Client
	unitID  uuid.UUID
	ethAddr *common.Address
}

// NewDataSender instantiates new data sender, does not create a connection to broker
func NewDataSender(unitID uuid.UUID, addr *common.Address) DataSender {
	// Setup mqtt connection. Does not connect
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	return &dataSender{
		client:  client,
		unitID:  unitID,
		ethAddr: addr,
	}
}

func (ds *dataSender) SendFingerprintData(data FingerprintData) error {
	ceh := newCloudEventHeaders(ds.unitID, ds.ethAddr)
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

func (ds *dataSender) SendErrorsData(data ErrorsData) error {
	ceh := newCloudEventHeaders(ds.unitID, ds.ethAddr)
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
	token := ds.client.Publish(topic, 0, false, string(payload))
	token.Wait() // just waits up until message goes through

	// Check if the message was successfully published
	if token.Error() != nil {
		return errors.Wrap(err, "Failed to publish MQTT message")
	}

	return nil
}

func signPayload(payload []byte, unitID uuid.UUID) ([]byte, error) {
	keccak256Hash := crypto.Keccak256Hash(payload)
	sig, err := commands.SignHash(unitID, keccak256Hash.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign the status update")
	}
	// note the path should match the CloudEventHeaders signature name
	payload, err = sjson.SetBytes(payload, "signature", hex.EncodeToString(sig))
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

func newCloudEventHeaders(unitID uuid.UUID, ethAddress *common.Address) CloudEventHeaders {
	ce := CloudEventHeaders{
		ID:          ksuid.New().String(),
		Source:      "autopi/status/fingerprint",
		SpecVersion: "1.0",
		Subject:     unitID.String(),
		Time:        time.Now().UTC(),
		Type:        "zone.dimo.device.status.fingerprint", // should we change this for errors?
	}
	if ethAddress != nil {
		ce.DeviceAddress = ethAddress.Hex()
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
	// DeviceAddress is an extension, for the ethereum address of the device
	DeviceAddress string `json:"deviceaddress"`
}

type DeviceFingerprintCloudEvent struct {
	CloudEventHeaders
	Data FingerprintData `json:"data"`
}

type FingerprintData struct {
	Vin            string  `json:"vin"`
	Protocol       string  `json:"protocol"`
	Odometer       float64 `json:"odometer,omitempty"`
	RpiUptimeSecs  int     `json:"rpiUptimeSecs,omitempty"`
	BatteryVoltage float64 `json:"batteryVoltage,omitempty"`
}

type DeviceErrorsCloudEvent struct {
	CloudEventHeaders
	Data ErrorsData `json:"data"`
}

type ErrorsData struct {
	Errors         []string `json:"errors"`
	RpiUptimeSecs  int      `json:"rpiUptimeSecs,omitempty"`
	BatteryVoltage float64  `json:"batteryVoltage,omitempty"`
}
