package internal

import (
	"fmt"
	"time"

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
	// read any existing settings
	var pidsConfig *models.TemplatePIDs
	var deviceSettings *models.TemplateDeviceSettings
	templateURLsLocal, err := vt.lss.ReadTemplateURLs()
	if err != nil {
		vt.logger.Err(err).Msg("could not read local settings for template URLs, continuing")
	} else {
		pidsConfig, err = vt.lss.ReadPIDsConfig()
		if err != nil {
			vt.logger.Err(err).Msg("could not read local settings for PIDs configs, continuing")
		}
		deviceSettings, err = vt.lss.ReadTemplateDeviceSettings()
		if err != nil {
			vt.logger.Err(err).Msg("could not read local settings for device settings, continuing")
		}
	}

	templateURLsRemote, err := gateways.Retry[models.TemplateURLs](3, 1*time.Second, vt.logger, func() (interface{}, error) {
		return vt.vsd.GetUrlsByEthAddr(addr)
	})

	if err != nil || templateURLsRemote == nil {
		vt.logger.Info().Msgf("unable to get template urls by eth addr:%s", addr.String())
	}
	// at this point, if have not local settings, and templateURLsRemote are empty from local settings, abort mission.
	if templateURLsLocal == nil && templateURLsRemote == nil {
		// todo - this is exagerated, let's return default settings, and just pidsConfig variable
		return nil, nil, fmt.Errorf("could not get template URL settings from remote, or from local store, cannot proceed")
	}
	// if can't get nothing from remote, just return what we got locally
	if templateURLsRemote == nil {
		return pidsConfig, deviceSettings, nil
	}
	// if no change, just return what we have
	if templateURLsLocal != nil &&
		templateURLsRemote.PidURL == templateURLsLocal.PidURL &&
		templateURLsRemote.DeviceSettingURL == templateURLsLocal.DeviceSettingURL && deviceSettings != nil {
		vt.logger.Info().Msg("vehicle template configuration has not changed, keeping current.")
		return pidsConfig, deviceSettings, nil
	}
	// if we get here, means version are different and we must retrieve and update, or we have nothing recent saved locally
	saveUrlsErr := vt.lss.WriteTemplateURLs(*templateURLsRemote)
	if saveUrlsErr != nil {
		vt.logger.Err(saveUrlsErr).Msgf("failed to save template urls %+v", *templateURLsRemote)
	}
	// PIDs, device settings, DBC (leave for later). If we can't get any of them, return what we have locally
	remotePids, err := gateways.Retry[models.TemplatePIDs](3, 1*time.Second, vt.logger, func() (interface{}, error) {
		return vt.vsd.GetPIDs(templateURLsRemote.PidURL)
	})
	if err != nil {
		vt.logger.Err(err).Msgf("could not get pids from api url: %s", templateURLsRemote.PidURL)
	} else {
		pidsConfig = remotePids
		err = vt.lss.WritePIDsConfig(*remotePids)
		if err != nil {
			vt.logger.Err(err).Msgf("failed to write pids config locally %+v", *remotePids)
		}
	}
	// get device settings
	deviceSettings, err = gateways.Retry[models.TemplateDeviceSettings](3, 1*time.Second, vt.logger, func() (interface{}, error) {
		return vt.vsd.GetDeviceSettings(templateURLsRemote.DeviceSettingURL)
	})
	if err != nil {
		vt.logger.Err(err).Msgf("could not get settings from api url: %s", templateURLsRemote.DeviceSettingURL)
	}

	return pidsConfig, deviceSettings, nil
}
