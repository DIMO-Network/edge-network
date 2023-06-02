package service

import (
	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
)

// Confirm This method doesn't expect a reply so it is just a
// confirmation that value was received.
//
// Possible Errors: org.bluez.Error.Failed
func (c *Char) Confirm() *dbus.Error {
	log.Debug("Char.Confirm")
	return nil
}

// StartNotify Starts a notification session from this characteristic
// if it supports value notifications or indications.
//
// Possible Errors: org.bluez.Error.Failed
//
//	org.bluez.Error.NotPermitted
//	org.bluez.Error.InProgress
//	org.bluez.Error.NotSupported
func (c *Char) StartNotify() *dbus.Error {
	log.Debug("Char.StartNotify")
	return nil
}

// StopNotify This method will cancel any previous StartNotify
// transaction. Note that notifications from a
// characteristic are shared between sessions thus
// calling StopNotify will release a single session.
//
// Possible Errors: org.bluez.Error.Failed
func (c *Char) StopNotify() *dbus.Error {
	log.Debug("Char.StopNotify")
	return nil
}

// ReadValue Issues a request to read the value of the
// characteristic and returns the value if the
// operation was successful.
//
// Possible options: "offset": uint16 offset
//
//	"device": Object Device (Server only)
//
// Possible Errors: org.bluez.Error.Failed
//
//	org.bluez.Error.InProgress
//	org.bluez.Error.NotPermitted
//	org.bluez.Error.NotAuthorized
//	org.bluez.Error.InvalidOffset
//	org.bluez.Error.NotSupported
func (c *Char) ReadValue(options map[string]interface{}) ([]byte, *dbus.Error) {

	log.Debug("Characteristic.ReadValue")
	if c.readCallback != nil {
		b, err := c.readCallback(c, options)
		if err != nil {
			return nil, dbus.MakeFailedError(err)
		}
		return b, nil
	}

	return c.Properties.Value, nil
}

// WriteValue Issues a request to write the value of the
// characteristic.
//
// Possible options: "offset": Start offset
//
//	"device": Device path (Server only)
//	"link": Link type (Server only)
//	"prepare-authorize": boolean Is prepare
//					 authorization
//					 request
//
// Possible Errors: org.bluez.Error.Failed
//
//	org.bluez.Error.InProgress
//	org.bluez.Error.NotPermitted
//	org.bluez.Error.InvalidValueLength
//	org.bluez.Error.NotAuthorized
//	org.bluez.Error.NotSupported
func (c *Char) WriteValue(value []byte, _ map[string]interface{}) *dbus.Error {

	log.Trace("Characteristic.WriteValue")

	val := value
	if c.writeCallback != nil {
		log.Trace("Used write callback")
		b, err := c.writeCallback(c, value)
		val = b
		if err != nil {
			return dbus.MakeFailedError(err)
		}
	} else {
		log.Trace("Store directly to value (no callback)")
	}

	// TODO update on Properties interface
	c.Properties.Value = val
	err := c.iprops.Instance().Set(c.Interface(), "Value", dbus.MakeVariant(value))

	return err
}
