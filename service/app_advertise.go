package service

import (
	"log"

	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
)

func (app *App) GetAdvertisement() *advertising.LEAdvertisement1Properties {
	return app.advertisement
}

func (app *App) Advertise(timeout uint32, localName string, advertisedServices []string) (func(), error) {

	adv := app.GetAdvertisement()

	adv.Timeout = uint16(timeout)
	adv.Duration = uint16(timeout)
	adv.LocalName = localName
	adv.ServiceUUIDs = advertisedServices
	cancel, err := api.ExposeAdvertisement(app.adapterID, adv, timeout)

	if err != nil {
		log.Fatalf("Failed advertising: %s", err)
	}
	return cancel, err
}
