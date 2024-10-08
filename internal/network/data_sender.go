package network

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/config"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/shared"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// it is the responsibility of the DataSender to determine what topic to use
const canDumpTopic = "protocol/canbus/dump"

//go:generate mockgen -source data_sender.go -destination mocks/data_sender_mock.go
type DataSender interface {
	SendErrorPayload(err error, powerStatus *api.PowerStatusResponse) error
	SendLogsData(data models.ErrorsData) error
	// SendFingerprintData sends VIN and protocol over mqtt to corresponding topic, could add anything else to help identify vehicle
	SendFingerprintData(data models.FingerprintData) error
	SendCanDumpData(data models.CanDumpData) error
	// SendDeviceStatusData sends queried vehicle data over mqtt, per configuration from vehicle-signal-decoding api.
	// The data can be gzip compressed or not
	SendDeviceStatusData(data any) error
	// SendDeviceNetworkData sends queried network data over mqtt to a separate network topic
	SendDeviceNetworkData(data models.DeviceNetworkData) error
	// SetVehicleInfo sets the vehicle info for the data sender
	SetVehicleInfo(vehicleInfo models.VehicleInfo)
}

type dataSender struct {
	client      mqtt.Client
	unitID      uuid.UUID
	ethAddr     common.Address
	logger      zerolog.Logger
	mqtt        config.Mqtt
	vehicleInfo models.VehicleInfo
}

func (ds *dataSender) SetVehicleInfo(vehicleInfo models.VehicleInfo) {
	ds.vehicleInfo = vehicleInfo
}

// NewDataSender instantiates new data sender, does not create a connection to broker
func NewDataSender(unitID uuid.UUID, addr common.Address, logger zerolog.Logger, vehicleInfo models.VehicleInfo, conf config.Config) DataSender {
	client := setupMqttConnection(conf, addr, logger)

	return &dataSender{
		client:      client,
		unitID:      unitID,
		ethAddr:     addr,
		logger:      logger,
		mqtt:        conf.Mqtt,
		vehicleInfo: vehicleInfo,
	}
}

// setupMqttConnection establishes a connection to the MQTT broker based on the provided configuration.
// The function configures the MQTT client options, sets up the file store for message buffering,
// and handles the TLS configuration if a secure connection is required.
// If the connection fails, an error message is logged, but the function still returns the client.
func setupMqttConnection(conf config.Config, addr common.Address, logger zerolog.Logger) mqtt.Client {
	// Setup mqtt connection.
	isSecureConn := conf.Mqtt.Broker.TLS.Enabled

	// offline buffering settings
	b := conf.Mqtt.Client.Buffering

	// Set the logger for the MQTT client. Uncomment to enable debug logging
	//mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)

	// Create MQTT client options
	opts := mqtt.NewClientOptions()
	opts.AddBroker(conf.Mqtt.Broker.Host + ":" + strconv.Itoa(conf.Mqtt.Broker.Port))
	// this will allow to publish messages even if connection is not established and saving messages in file store.
	// with waitWithTimeout in place,  on connect failure, we still can publish messages which would be stored in fileStore
	opts.SetConnectRetry(true)
	// messages buffering in file store, default is "in memory" store for Qos1 and 2
	fileStore := mqtt.NewFileStore(b.FileStore)
	store := &CustomFileStore{FileStore: fileStore, limit: b.Limit}
	opts.SetStore(store)
	// indicates that the client should store the messages in the file store after shutdown and pickup messages upon start up from the disk
	// if we shut down the edge-network and start again, all buffered messages will be deleted on startup unless we set SetCleanSession to false
	opts.SetCleanSession(b.CleanSession)
	// ConnectRetryInterval is the time to wait between reconnection attempts. Default is  30 sec
	opts.SetConnectRetryInterval(time.Second * time.Duration(b.ConnectRetryInterval))
	// If we are using SetCleanSession=false, we need to specify client-id
	opts.SetClientID(addr.String())

	if isSecureConn {
		// Load CA certificate
		caCert, err := os.ReadFile("/opt/autopi/root_cert_bundle.crt")
		if err != nil {
			logger.Error().Err(err).Msg("failed to read CA certificate")
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		// Load client certificate and private key
		cert, err := tls.LoadX509KeyPair(conf.Services.Ca.CertPath, conf.Services.Ca.PrivateKeyPath)
		if err != nil {
			logger.Error().Err(err).Msg("failed to load client certificate and private key")
		}

		// Create TLS configuration
		tlsConfig := &tls.Config{
			RootCAs:      caCertPool,
			Certificates: []tls.Certificate{cert},
		}

		// Create MQTT client options with TLS configuration
		opts.SetTLSConfig(tlsConfig)
	}
	// Create and start a client
	client := mqtt.NewClient(opts)
	// with waitWithTimeout on connect failure, we are not blocked(opposite to Wait()) and can publish messages which would be stored in fileStore
	if token := client.Connect(); token.WaitTimeout(time.Second*5) && token.Error() != nil {
		logger.Error().Err(token.Error()).Msg("failed to connect to mqtt broker")
	}
	return client
}

func (ds *dataSender) SendFingerprintData(data models.FingerprintData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}
	ce := shared.CloudEvent[models.FingerprintData]{
		ID:          ksuid.New().String(),
		Source:      "aftermarket/device/fingerprint",
		SpecVersion: "1.0",
		Subject:     ds.ethAddr.Hex(),
		Time:        time.Now().UTC(),
		Type:        "zone.dimo.aftermarket.device.fingerprint",
		DataSchema:  "dimo.zone.status/v2.0",
		Data:        data,
	}
	payload, err := json.Marshal(ce)
	if err != nil {
		ds.logger.Error().Err(err).Msg("failed to marshall cloudevent")
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	fingerprint := fmt.Sprintf(ds.mqtt.Topics.Fingerprint, ce.Subject)

	err = ds.sendPayload(fingerprint, payload, false)
	if err != nil {
		ds.logger.Error().Err(err).Msg("failed send payload")
		return err
	}
	return nil
}

func (ds *dataSender) SendDeviceStatusData(data any) error {
	ce := models.DeviceDataStatusCloudEvent[any]{
		CloudEvent: shared.CloudEvent[any]{
			ID:          ksuid.New().String(),
			Source:      "dimo/integration/27qftVRWQYpVDcO5DltO5Ojbjxk",
			SpecVersion: "1.0",
			Subject:     ds.ethAddr.Hex(),
			Time:        time.Now().UTC(),
			Type:        "com.dimo.device.status.v2",
			DataSchema:  "dimo.zone.status/v2.0",
			Data:        data,
		},
	}

	if ds.vehicleInfo.TokenID != 0 {
		ce.TokenID = ds.vehicleInfo.TokenID
	}

	payload, err := json.Marshal(ce)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	status := fmt.Sprintf(ds.mqtt.Topics.Status, ce.Subject)

	err = ds.sendPayload(status, payload, true)
	if err != nil {
		return err
	}
	return nil
}

func (ds *dataSender) SendDeviceNetworkData(data models.DeviceNetworkData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}

	ce := shared.CloudEvent[models.DeviceNetworkData]{
		ID:          ksuid.New().String(),
		Source:      "aftermarket/device/network",
		SpecVersion: "1.0",
		Subject:     ds.ethAddr.Hex(),
		Time:        time.Now().UTC(),
		Type:        "com.dimo.device.network",
		DataSchema:  "dimo.zone.status/v2.0",
		Data:        data,
	}
	payload, err := json.Marshal(ce)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	network := fmt.Sprintf(ds.mqtt.Topics.Network, ce.Subject)

	err = ds.sendPayload(network, payload, true)
	if err != nil {
		return err
	}
	return nil
}

func (ds *dataSender) SendCanDumpData(data models.CanDumpData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}
	ce := shared.CloudEvent[models.CanDumpData]{
		ID:          ksuid.New().String(),
		Source:      "aftermarket/device/canbus/dump",
		SpecVersion: "1.0",
		Subject:     ds.ethAddr.Hex(),
		Time:        time.Now().UTC(),
		Type:        "zone.dimo.aftermarket.canbus.dump",
		DataSchema:  "dimo.zone.status/v2.0",
		Data:        data,
	}
	println("Sending can dump data: (payload)")
	payload, err := json.Marshal(ce)
	println(payload)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	err = ds.sendPayload(canDumpTopic, payload, false)
	if err != nil {
		return err
	}
	return nil
}

func (ds *dataSender) SendLogsData(data models.ErrorsData) error {
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().UTC().UnixMilli()
	}

	// sending the serial number of the device
	if ds.unitID != uuid.Nil {
		data.Device.UnitID = ds.unitID.String()
		data.TokenID = ds.vehicleInfo.TokenID
		// todo think how to pass SoftwareVersion
		//data.Device.SoftwareVersion = "ds.vehicleInfo.SoftwareVersion"
	}

	ce := shared.CloudEvent[models.ErrorsData]{
		ID:          ksuid.New().String(),
		Source:      "aftermarket/device/logs",
		SpecVersion: "1.0",
		Subject:     ds.ethAddr.Hex(),
		Time:        time.Now().UTC(),
		Type:        "zone.dimo.aftermarket.device.logs",
		DataSchema:  "dimo.zone.status/v2.0",
		Data:        data,
	}
	payload, err := json.Marshal(ce)
	if err != nil {
		return errors.Wrap(err, "failed to marshall cloudevent")
	}

	logs := fmt.Sprintf(ds.mqtt.Topics.Logs, ce.Subject)

	err = ds.sendPayload(logs, payload, true)
	if err != nil {
		return err
	}
	return nil
}

// SendPayload connects to broker and sends a filled in status update via mqtt to broker address, should already be in json format
func (ds *dataSender) sendPayload(topic string, payload []byte, compress bool) error {
	// todo: determine if we want to be connecting and disconnecting from mqtt broker for every status update we send (when start sending more periodic data besides VIN)
	if !gjson.GetBytes(payload, "subject").Exists() {
		return fmt.Errorf("payload did not have expected subject cloud event property")
	}

	// signature for the payload
	payload, err := ds.signPayload(payload, ds.unitID)
	if err != nil {
		return err
	}

	// Compress the payload
	if compress {
		compressedPayload, err := compressPayload(payload)
		if err != nil {
			return errors.Wrap(err, "Failed to compress device status data")
		}
		payload, err = json.Marshal(compressedPayload)
		if err != nil {
			return errors.Wrap(err, "failed to marshall compressedPayload")
		}
	}

	// Publish the MQTT message
	token := ds.client.Publish(topic, 1, false, payload)
	// we should not wait for the message to be ack by broker, as this is blocking call
	//  token.Wait() do not blocking Qos 0, only Qos 1 and Qos 2 is blocking. So if we want to us Qos 1, we should not use token.Wait() after each publish
	// we will use file store to buffer the messages, and when broker is up, it will send buffered messages on Connection
	//token.Wait() // just waits up until message goes through

	// Check if the message was successfully published
	if token.Error() != nil {
		return errors.Wrap(err, "Failed to publish MQTT message")
	}

	ds.logger.Debug().Msgf("sending mqtt payload to topic: %s with payload: %s", topic, string(payload))

	return nil
}

// DeviceStatusData is formatted as json, gzip compressed, then base64 compressed.
// This is done to reduce the size of the payload sent to the cloud over MQTT.
func compressPayload(payload []byte) (*models.CompressedPayload, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(payload)
	_ = gz.Close()

	if err != nil {
		return nil, err
	}

	compressedData := &models.CompressedPayload{
		Payload: base64.StdEncoding.EncodeToString(buf.Bytes()),
	}
	return compressedData, nil
}

func (ds *dataSender) signPayload(payload []byte, unitID uuid.UUID) ([]byte, error) {
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
	data := models.ErrorsData{}
	if powerStatus != nil {
		data.Device.BatteryVoltage = powerStatus.Spm.Battery.Voltage
		data.Device.RpiUptimeSecs = powerStatus.Rpi.Uptime.Seconds
	}
	if ds.vehicleInfo.VehicleDefinition.Model != "" {
		data.ModelSlug = shared.SlugString(ds.vehicleInfo.VehicleDefinition.Model)
	}
	data.Errors = append(data.Errors, err.Error())

	return ds.SendLogsData(data)
}
