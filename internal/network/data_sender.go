package network

import (
	"encoding/hex"
	"encoding/json"
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
const topic = "raw"
const broker = "tcp://localhost:1883"

//go:generate mockgen -source data_sender.go -destination mocks/data_sender_mock.go
type DataSender interface {
	SendPayload(status *StatusUpdatePayload, unitID uuid.UUID) error
	SendErrorPayload(unitID uuid.UUID, ethAddress *common.Address, err error) error
}

type dataSender struct {
	client mqtt.Client
}

// NewDataSender instantiates new data sender, does not create a connection to broker
func NewDataSender() DataSender {
	// Setup mqtt connection. Does not connect
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)
	return &dataSender{
		client: client,
	}
}

// SendPayload connects to broker and sends a filled in status update via mqtt to broker address
func (ds *dataSender) SendPayload(status *StatusUpdatePayload, unitID uuid.UUID) error {
	// todo: determine if we want to be connecting and disconnecting from mqtt broker for every status update we send
	status.SerialNumber = unitID.String()

	payload, err := json.Marshal(status)
	if err != nil {
		return err
	}
	log.Infof("sending payload:\n")
	log.Infof("%s", string(payload))

	// Connect to the MQTT broker
	if token := ds.client.Connect(); token.Wait() && token.Error() != nil {
		return errors.Wrap(err, "failed to connect to mqtt broker:")
	}

	// Wait for the connection to be established
	for !ds.client.IsConnected() {
		time.Sleep(100 * time.Millisecond)
		// todo timeout?
	}
	// Disconnect from the MQTT broker
	defer ds.client.Disconnect(250)

	// signature for the payload
	keccak256Hash := crypto.Keccak256Hash(payload)
	sig, err := commands.SignHash(unitID, keccak256Hash.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to sign the status update")
	}

	// Publish the MQTT message
	payload, err = sjson.SetBytes(payload, "signature", hex.EncodeToString(sig)) // todo is this how we want the signature in the json?
	if err != nil {
		return errors.Wrap(err, "failed to add signature to status update")
	}
	token := ds.client.Publish(topic, 0, false, string(payload))
	token.Wait() // just waits up until message goes through

	// Check if the message was successfully published
	if token.Error() != nil {
		return errors.Wrap(err, "Failed to publish MQTT message")
	}

	return nil
}

type StatusUpdatePayload struct {
	// Timestamp the signal timestamp, in unix millis
	Timestamp int64 `json:"timestamp"`
	// SerialNumber is the autopi unit id
	SerialNumber    string           `json:"serial_number"`
	Data            StatusUpdateData `json:"data"`
	EthereumAddress string           `json:"ethereum_address"`
	Errors          []string         `json:"errors,omitempty"`
}

type StatusUpdateData struct {
	Vin      string  `json:"vin"`
	Protocol string  `json:"protocol"` // todo should we just post this to endpoint in vehicle-signal-decoding api, same with the VIN query
	Odometer float64 `json:"odometer,omitempty"`
}

func (ds *dataSender) SendErrorPayload(unitID uuid.UUID, ethAddress *common.Address, err error) error {
	payload := NewStatusUpdatePayload(unitID, ethAddress)
	payload.Errors = append(payload.Errors, err.Error())

	return ds.SendPayload(&payload, unitID)
}

func NewStatusUpdatePayload(unitID uuid.UUID, ethAddress *common.Address) StatusUpdatePayload {
	payload := StatusUpdatePayload{
		SerialNumber: unitID.String(),
		Timestamp:    time.Now().UTC().UnixMilli(),
	}
	if ethAddress != nil {
		payload.EthereumAddress = ethAddress.Hex()
	}
	return payload
}
