package internal

import (
	"encoding/json"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"
)

const topic = "reactor"

// SendPayload sends a filled in status update via mqtt to localhost server
func SendPayload(status *StatusUpdatePayload) error {
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

	// Publish the MQTT message
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
	Subject string           `json:"subject"`
	Data    StatusUpdateData `json:"data"`
}

type StatusUpdateData struct {
	Device StatusUpdateDevice `json:"meta"`

	VinTest      string `json:"canbus_vin_test"`
	ProtocolTest string `json:"canbus_protocol_test"`

	Signals StatusUpdateSignals `json:"signals"`
}

type StatusUpdateDevice struct {
	// Timestamp the signal timestamp, in unix millis
	Timestamp int64 `json:"timestamp"`
	// UnitID is the autopi unit id
	UnitID string `json:"unit_id"`
}

// StatusUpdateSignals not sure if need this but jic
type StatusUpdateSignals struct {
	VinTest      StringSignal `json:"canbus_vin_test"`
	ProtocolTest StringSignal `json:"canbus_protocol_test"`
}

type StringSignal struct {
	Value string `json:"value"`
}
