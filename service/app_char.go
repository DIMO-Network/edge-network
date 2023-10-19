package service

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez"
	"github.com/muka/go-bluetooth/bluez/profile/gatt"
	"github.com/rs/zerolog"
)

type CharReadCallback func(c *Char, options map[string]interface{}) ([]byte, error)
type CharWriteCallback func(c *Char, value []byte) ([]byte, error)

type Char struct {
	UUID    string
	app     *App
	service *Service

	path  dbus.ObjectPath
	descr map[dbus.ObjectPath]*Descr

	Properties *gatt.GattCharacteristic1Properties
	iprops     *api.DBusProperties

	readCallback  CharReadCallback
	writeCallback CharWriteCallback
	logger        zerolog.Logger
}

func (c *Char) Path() dbus.ObjectPath {
	return c.path
}

func (c *Char) DBusProperties() *api.DBusProperties {
	return c.iprops
}

func (c *Char) Interface() string {
	return gatt.GattCharacteristic1Interface
}

func (c *Char) GetProperties() bluez.Properties {
	descr := []dbus.ObjectPath{}
	for dpath := range c.descr {
		descr = append(descr, dpath)
	}
	c.Properties.Descriptors = descr
	c.Properties.Service = c.Service().Path()

	return c.Properties
}

func (c *Char) GetDescr() map[dbus.ObjectPath]*Descr {
	return c.descr
}

func (c *Char) App() *App {
	return c.app
}

func (c *Char) Service() *Service {
	return c.service
}

func (c *Char) DBusObjectManager() *api.DBusObjectManager {
	return c.App().DBusObjectManager()
}

func (c *Char) DBusConn() *dbus.Conn {
	return c.App().DBusConn()
}

func (c *Char) RemoveDescr(descr *Descr) error {

	if _, ok := c.descr[descr.Path()]; !ok {
		return nil
	}

	err := descr.Remove()
	if err != nil {
		return err
	}

	delete(c.descr, descr.Path())

	return nil
}

// Expose char to dbus
func (c *Char) Expose() error {
	return api.ExposeDBusService(c)
}

// Remove char from dbus
func (c *Char) Remove() error {
	return api.RemoveDBusService(c)
}

// NewDescr Init new descr
func (c *Char) NewDescr(uuid string) (*Descr, error) {

	descr := new(Descr)
	descr.UUID = c.App().GenerateUUID(uuid)

	descr.app = c.App()
	descr.char = c
	descr.Properties = NewGattDescriptor1Properties(descr.UUID)
	descr.path = dbus.ObjectPath(
		fmt.Sprintf("%s/descriptor%d", c.Path(), len(c.GetDescr())),
	)
	iprops, err := api.NewDBusProperties(c.App().DBusConn())
	if err != nil {
		return nil, err
	}
	descr.iprops = iprops

	return descr, nil
}

// AddDescr Add descr to dbus
func (c *Char) AddDescr(descr *Descr) error {

	err := api.ExposeDBusService(descr)
	if err != nil {
		return err
	}

	c.descr[descr.Path()] = descr

	err = c.DBusObjectManager().AddObject(descr.Path(), map[string]bluez.Properties{
		descr.Interface(): descr.GetProperties(),
	})
	if err != nil {
		return err
	}

	// update OM char too
	err = c.DBusObjectManager().AddObject(c.Path(), map[string]bluez.Properties{
		c.Interface(): c.GetProperties(),
	})
	if err != nil {
		return err
	}

	c.logger.Trace().Msgf("Added GATT Descriptor UUID=%s %s", descr.UUID, descr.Path())

	err = c.App().ExportTree()
	return err
}

// OnRead Set the Read callback, called when a client attempt to read
func (c *Char) OnRead(fx CharReadCallback) *Char {
	c.readCallback = fx
	return c
}

// OnWrite Set the Write callback, called when a client attempt to write
func (c *Char) OnWrite(fx CharWriteCallback) *Char {
	c.writeCallback = fx
	return c
}
