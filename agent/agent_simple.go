package agent

import (
	"fmt"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var agentInstances = 0

const AgentBasePath = "/agent/simple%d"
const SimpleAgentPinCode = "0000"
const SimpleAgentPassKey uint32 = 1024

func NextAgentPath() dbus.ObjectPath {
	p := dbus.ObjectPath(fmt.Sprintf(AgentBasePath, agentInstances))
	agentInstances += 1
	return p
}

// NewDefaultSimpleAgent return a SimpleAgent instance with default pincode and passcode
func NewDefaultSimpleAgent() *SimpleAgent {
	ag := &SimpleAgent{
		path:    NextAgentPath(),
		passKey: SimpleAgentPassKey,
		pinCode: SimpleAgentPinCode,
	}
	logrus.SetLevel(logrus.TraceLevel)
	return ag
}

// NewSimpleAgent return a SimpleAgent instance
func NewSimpleAgent() *SimpleAgent {
	ag := &SimpleAgent{
		path: NextAgentPath(),
	}
	return ag
}

// SimpleAgent implement interface Agent1Client
type SimpleAgent struct {
	path    dbus.ObjectPath
	pinCode string
	passKey uint32
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

	log.Debugf("SimpleAgent: RequestPinCode: %s", path)

	adapterID, err := adapter.ParseAdapterID(path)
	if err != nil {
		log.Warnf("SimpleAgent.RequestPinCode: Failed to load adapter %s", err)
		return "", dbus.MakeFailedError(err)
	}

	err = SetTrusted(adapterID, path)
	if err != nil {
		log.Errorf("SimpleAgent.RequestPinCode SetTrusted failed: %s", err)
		return "", dbus.MakeFailedError(err)
	}

	log.Debugf("SimpleAgent: Returning pin code: %s", simpleAgent.pinCode)
	return simpleAgent.pinCode, nil
}

func (simpleAgent *SimpleAgent) DisplayPinCode(device dbus.ObjectPath, pincode string) *dbus.Error {
	log.Info(fmt.Sprintf("SimpleAgent: DisplayPinCode (%s, %s)", device, pincode))
	return nil
}

func (simpleAgent *SimpleAgent) RequestPasskey(path dbus.ObjectPath) (uint32, *dbus.Error) {

	adapterID, err := adapter.ParseAdapterID(path)
	if err != nil {
		log.Warnf("SimpleAgent.RequestPassKey: Failed to load adapter %s", err)
		return 0, dbus.MakeFailedError(err)
	}

	err = SetTrusted(adapterID, path)
	if err != nil {
		log.Errorf("SimpleAgent.RequestPassKey: SetTrusted %s", err)
		return 0, dbus.MakeFailedError(err)
	}

	log.Debugf("RequestPasskey: returning %d", simpleAgent.passKey)
	return simpleAgent.passKey, nil
}

func (simpleAgent *SimpleAgent) DisplayPasskey(device dbus.ObjectPath, passkey uint32, entered uint16) *dbus.Error {
	log.Debugf("SimpleAgent: DisplayPasskey %s, %06d entered %d", device, passkey, entered)

	_, unitId := commands.GetDeviceName()

	err := commands.AnnounceCode(unitId, passkey)
	if err != nil {
		log.Warnf("Unable to announce the pairing code %s", err)
	}

	err = commands.AnnounceCode(unitId, passkey)
	if err != nil {
		log.Warnf("Unable to announce the pairing code %s", err)
	}

	err = commands.AnnounceCode(unitId, passkey)
	if err != nil {
		log.Warnf("Unable to announce the pairing code %s", err)
	}

	return nil
}

func (simpleAgent *SimpleAgent) RequestConfirmation(path dbus.ObjectPath, passkey uint32) *dbus.Error {

	log.Debugf("SimpleAgent: RequestConfirmation (%s, %06d)", path, passkey)
	_, unitId := commands.GetDeviceName()

	err := commands.ExtendSleepTimer(unitId)
	if err != nil {
		log.Warnf("Unable to extend sleep timer %s", err)
	}

	adapterID, err := adapter.ParseAdapterID(path)
	if err != nil {
		log.Warnf("SimpleAgent: Failed to load adapter %s", err)
		return dbus.MakeFailedError(err)
	}

	err = SetTrusted(adapterID, path)
	if err != nil {
		log.Warnf("Failed to set trust for %s: %s", path, err)
		return dbus.MakeFailedError(err)
	}

	log.Debug("SimpleAgent: RequestConfirmation OK")
	log.Debug("SimpleAgent: Extending sleep timer by 15 minutes.")
	return nil
}

func (simpleAgent *SimpleAgent) RequestAuthorization(device dbus.ObjectPath) *dbus.Error {
	log.Debugf("SimpleAgent: RequestAuthorization (%s)", device)
	return nil
}

func (simpleAgent *SimpleAgent) AuthorizeService(device dbus.ObjectPath, uuid string) *dbus.Error {
	log.Debugf("SimpleAgent: AuthorizeService (%s, %s)", device, uuid) // directly authorized
	return nil
}

func (simpleAgent *SimpleAgent) Cancel() *dbus.Error {
	log.Debugf("SimpleAgent: Cancel")
	return nil
}
