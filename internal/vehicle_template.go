package internal

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/gateways"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
)

// VehicleTemplates idea is to be a layer on top of calling the gateway for VehicleSignalDecoding, that handles
// caching of configs - checking if there is an updated version of the templates available
type VehicleTemplates interface {
	GetTemplateSettings(addr *common.Address) (*models.TemplatePIDs, *models.TemplateDeviceSettings, error)
}

type vehicleTemplates struct {
	logger zerolog.Logger
	vsd    gateways.VehicleSignalDecoding
	lss    loggers.TemplateStore
}

func NewVehicleTemplates(logger zerolog.Logger, vsd gateways.VehicleSignalDecoding, lss loggers.TemplateStore) VehicleTemplates {
	return &vehicleTemplates{logger: logger, vsd: vsd, lss: lss}
}

// GetTemplateSettings checks for any new template settings and if so updates the local settings, returning the latest
// settings. Logs if encounters errors along the way. Continues and gets local settings if can't get anything from remote. Errors if can't get anything useful.
func (vt *vehicleTemplates) GetTemplateSettings(addr *common.Address) (*models.TemplatePIDs, *models.TemplateDeviceSettings, error) {
	vinConfig, err := vt.lss.ReadVINConfig() // should this be more of VIN info? stored VIN info
	if err != nil {
		vt.logger.Err(err).Msg("could not read local settings for stored VIN, continuing")
	}
	templateURLsLocal, err := vt.lss.ReadTemplateURLs()
	if err != nil {
		vt.logger.Err(err).Msg("could not read local settings for template URLs, continuing")
	}
	// read any existing settings
	pidsConfig, err := vt.lss.ReadPIDsConfig()
	if err != nil {
		vt.logger.Err(err).Msg("could not read local settings for PIDs configs, continuing")
	}
	deviceSettings, err := vt.lss.ReadTemplateDeviceSettings()
	if err != nil {
		vt.logger.Err(err).Msg("could not read local settings for device settings, continuing")
	}

	var templateURLsRemote *models.TemplateURLs
	templateURLsRemote, err = vt.vsd.GetUrlsByEthAddr(addr)
	if err != nil {
		vt.logger.Err(err).Msg("unable to get template urls by eth addr, trying by VIN next")
		if vinConfig != nil && len(vinConfig.VIN) == 17 {
			templateURLsRemote, err = vt.vsd.GetUrlsByVin(vinConfig.VIN)
			if err != nil {
				vt.logger.Err(err).Msg("unable to get template urls by vin")
			}
		}
	}
	// at this point, if have not local settings, and templateURLsRemote are empty from local settings, abort mission.
	if templateURLsLocal == nil && templateURLsRemote == nil {
		return nil, nil, fmt.Errorf("could not get template URL settings from remote, or from local store, cannot proceed")
	}
	// if can't get nothing from remote, just return what we got locally
	if templateURLsRemote == nil {
		return pidsConfig, deviceSettings, nil
	}
	// if no change, just return what we have
	if templateURLsRemote.Version == templateURLsLocal.Version {
		vt.logger.Info().Msgf("vehicle template configuration has not changed, keeping current. version %s", templateURLsLocal.Version)
		return pidsConfig, deviceSettings, nil
	}
	// if we get here, means version are different and we must retrieve and update
	// PIDs, device settings, DBC (leave for later). If we can't get any of them, return what we have locally
	remotePids, err := vt.vsd.GetPIDs(templateURLsRemote.PidUrl)
	if err != nil {
		vt.logger.Err(err).Msgf("could not get pids from api url: %s", templateURLsRemote.PidUrl)
	} else {
		pidsConfig = remotePids
		err = vt.lss.WritePIDsConfig(*pidsConfig)
		if err != nil {
			vt.logger.Err(err).Msgf("failed to write pids config locally %+v", *pidsConfig)
		}
	}
	// get device settings
	settings, err := vt.vsd.GetDeviceSettings(templateURLsRemote.DeviceSettingUrl)
	if err != nil {
		vt.logger.Err(err).Msgf("could not get settings from api url: %s", templateURLsRemote.DeviceSettingUrl)
	} else {
		deviceSettings = settings
	}

	return pidsConfig, deviceSettings, nil
}
