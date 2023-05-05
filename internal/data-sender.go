package internal

import (
	"encoding/hex"
	"encoding/json"
	"github.com/DIMO-Network/edge-network/commands"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/sjson"
	"time"
)

// thought: should we have another topic for errors? ie. signals we could not get
const topic = "raw"

// SendPayload sends a filled in status update via mqtt to localhost server
func SendPayload(status *StatusUpdatePayload, unitID uuid.UUID) error {
	// todo: determin if we want to be connecting and disconnecting from mqtt broker for every status update we send

	payload, err := json.Marshal(status)
	if err != nil {
		return err
	}
	log.Infof("sending payload:\n")
	log.Infof("%s", string(payload))

	// Setup mqtt connection
	broker := "tcp://localhost:1883"
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	client := mqtt.NewClient(opts)

	// Connect to the MQTT broker
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return errors.Wrap(err, "Failed to connect to MQTT broker:")
	}

	// Wait for the connection to be established
	for !client.IsConnected() {
		time.Sleep(100 * time.Millisecond)
		// todo timeout?
	}
	// Disconnect from the MQTT broker
	defer client.Disconnect(250)

	// signature for the payload
	sig, err := commands.SignHash(unitID, payload)
	if err != nil {
		return errors.Wrap(err, "failed to sign the status update")
	}

	// Publish the MQTT message
	payload, err = sjson.SetBytes(payload, "signature", hex.EncodeToString(sig)) // todo is this how we want the signature in the json?
	if err != nil {
		return errors.Wrap(err, "failed to add signature to status update")
	}
	token := client.Publish(topic, 0, false, string(payload))
	token.Wait() // just waits up until message goes through

	// Check if the message was successfully published
	if token.Error() != nil {
		return errors.Wrap(err, "Failed to publish MQTT message")
	}

	return nil
}

type StatusUpdatePayload struct {
	// Subject here subject means autopi unit id (it will get converted after ingestion)
	Subject string `json:"subject"`
	// Timestamp the signal timestamp, in unix millis
	Timestamp int64 `json:"timestamp"`
	// UnitID is the autopi unit id
	UnitID          string           `json:"unit_id"`
	Data            StatusUpdateData `json:"data"`
	EthereumAddress string           `json:"ethereum_address"`
}

type StatusUpdateData struct {
	Vin      string  `json:"vin"`
	Protocol string  `json:"protocol"` // todo should we just post this to endpoint in vehicle-signal-decoding api
	Odometer float64 `json:"odometer,omitempty"`
}
