package agent

import (
	"fmt"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/rs/zerolog"
)

// todo doesn't seem any of this is used. Do we want to save it somewhere else or was it an experiment?

var agentInstances = 0

const AgentBasePath = "/agent/simple%d"
const SimpleAgentPinCode = "0000"
const SimpleAgentPassKey uint32 = 1024

func NextAgentPath() dbus.ObjectPath {
	p := dbus.ObjectPath(fmt.Sprintf(AgentBasePath, agentInstances))
	agentInstances++
	return p
}

// NewDefaultSimpleAgent return a SimpleAgent instance with default pincode and passcode
func NewDefaultSimpleAgent(logger zerolog.Logger) *SimpleAgent {
	ag := &SimpleAgent{
		path:    NextAgentPath(),
		passKey: SimpleAgentPassKey,
		pinCode: SimpleAgentPinCode,
		logger:  logger,
	}
	//logrus.SetLevel(logrus.InfoLevel) // modify to trace if need more detail
	return ag
}

// NewSimpleAgent return a SimpleAgent instance
func NewSimpleAgent(logger zerolog.Logger) *SimpleAgent {
	ag := &SimpleAgent{
		path:   NextAgentPath(),
		logger: logger,
	}
	return ag
}

// SimpleAgent implement interface Agent1Client
type SimpleAgent struct {
	path    dbus.ObjectPath
	pinCode string
	passKey uint32
	logger  zerolog.Logger
}

func (simpleAgent *SimpleAgent) SetPassKey(passkey uint32) {
	simpleAgent.passKey = passkey
}

func (simpleAgent *SimpleAgent) SetPassCode(pinCode string) {
	simpleAgent.pinCode = pinCode
}

func (simpleAgent *SimpleAgent) PassKey() uint32 {
	return simpleAgent.passKey
}

func (simpleAgent *SimpleAgent) PassCode() string {
	return simpleAgent.pinCode
}

func (simpleAgent *SimpleAgent) Path() dbus.ObjectPath {
	return simpleAgent.path
}

func (simpleAgent *SimpleAgent) Interface() string {
	return Agent1Interface
}

func (simpleAgent *SimpleAgent) Release() *dbus.Error {
	return nil
}

func (simpleAgent *SimpleAgent) RequestPinCode(path dbus.ObjectPath) (string, *dbus.Error) {

	simpleAgent.logger.Debug().Msgf("SimpleAgent: RequestPinCode: %s", path)

	adapterID, err := adapter.ParseAdapterID(path)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("SimpleAgent.RequestPinCode: Failed to load adapter %s", err)
		return "", dbus.MakeFailedError(err)
	}

	err = SetTrusted(adapterID, path, simpleAgent.logger)
	if err != nil {
		simpleAgent.logger.Error().Msgf("SimpleAgent.RequestPinCode SetTrusted failed: %s", err)
		return "", dbus.MakeFailedError(err)
	}

	simpleAgent.logger.Debug().Msgf("SimpleAgent: Returning pin code: %s", simpleAgent.pinCode)
	return simpleAgent.pinCode, nil
}

func (simpleAgent *SimpleAgent) DisplayPinCode(device dbus.ObjectPath, pincode string) *dbus.Error {
	simpleAgent.logger.Info().Msg(fmt.Sprintf("SimpleAgent: DisplayPinCode (%s, %s)", device, pincode))
	return nil
}

func (simpleAgent *SimpleAgent) RequestPasskey(path dbus.ObjectPath) (uint32, *dbus.Error) {

	adapterID, err := adapter.ParseAdapterID(path)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("SimpleAgent.RequestPassKey: Failed to load adapter %s", err)
		return 0, dbus.MakeFailedError(err)
	}

	err = SetTrusted(adapterID, path, simpleAgent.logger)
	if err != nil {
		simpleAgent.logger.Error().Msgf("SimpleAgent.RequestPassKey: SetTrusted %s", err)
		return 0, dbus.MakeFailedError(err)
	}

	simpleAgent.logger.Debug().Msgf("RequestPasskey: returning %d", simpleAgent.passKey)
	return simpleAgent.passKey, nil
}

func (simpleAgent *SimpleAgent) DisplayPasskey(device dbus.ObjectPath, passkey uint32, entered uint16) *dbus.Error {
	simpleAgent.logger.Debug().Msgf("SimpleAgent: DisplayPasskey %s, %06d entered %d", device, passkey, entered)
	_, unitID := commands.GetDeviceName(simpleAgent.logger)

	err := commands.ExtendSleepTimer(unitID)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("Unable to extend sleep timer %s", err)
	}

	err = commands.AnnounceCode(unitID, "Pin Code", passkey, simpleAgent.logger)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("Unable to announce the pairing code %s", err)
	}

	err = commands.AnnounceCode(unitID, "Repeating Pin Code", passkey, simpleAgent.logger)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("Unable to announce the pairing code %s", err)
	}

	err = commands.AnnounceCode(unitID, "Pin Code", passkey, simpleAgent.logger)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("Unable to announce the pairing code %s", err)
	}

	return nil
}

func (simpleAgent *SimpleAgent) RequestConfirmation(path dbus.ObjectPath, passkey uint32) *dbus.Error {

	simpleAgent.logger.Debug().Msgf("SimpleAgent: RequestConfirmation (%s, %06d)", path, passkey)
	_, unitID := commands.GetDeviceName(simpleAgent.logger)

	err := commands.ExtendSleepTimer(unitID)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("Unable to extend sleep timer %s", err)
	}

	adapterID, err := adapter.ParseAdapterID(path)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("SimpleAgent: Failed to load adapter %s", err)
		return dbus.MakeFailedError(err)
	}

	err = SetTrusted(adapterID, path, simpleAgent.logger)
	if err != nil {
		simpleAgent.logger.Warn().Msgf("Failed to set trust for %s: %s", path, err)
		return dbus.MakeFailedError(err)
	}

	simpleAgent.logger.Debug().Msg("SimpleAgent: RequestConfirmation OK")
	simpleAgent.logger.Debug().Msg("SimpleAgent: Extending sleep timer by 15 minutes.")
	return nil
}

func (simpleAgent *SimpleAgent) RequestAuthorization(device dbus.ObjectPath) *dbus.Error {
	simpleAgent.logger.Debug().Msgf("SimpleAgent: RequestAuthorization (%s)", device)
	return nil
}

func (simpleAgent *SimpleAgent) AuthorizeService(device dbus.ObjectPath, uuid string) *dbus.Error {
	simpleAgent.logger.Debug().Msgf("SimpleAgent: AuthorizeService (%s, %s)", device, uuid) // directly authorized
	return nil
}

func (simpleAgent *SimpleAgent) Cancel() *dbus.Error {
	simpleAgent.logger.Debug().Msgf("SimpleAgent: Cancel")
	return nil
}
