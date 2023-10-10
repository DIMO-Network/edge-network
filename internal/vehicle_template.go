package internal

import (
	"github.com/DIMO-Network/edge-network/internal/gateways"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type VehicleTemplates interface {
	GetTemplateSettings(vin string, addr *common.Address) (*loggers.PIDLoggerSettings, error)
}

type vehicleTemplates struct {
	logger zerolog.Logger
	vsd    gateways.VehicleSignalDecodingAPIService
	lss    loggers.TemplateStore
}

func NewVehicleTemplates(logger zerolog.Logger, vsd gateways.VehicleSignalDecodingAPIService, lss loggers.TemplateStore) VehicleTemplates {
	return &vehicleTemplates{logger: logger, vsd: vsd, lss: lss}
}

// GetTemplateSettings checks for any new template settings and if so updates the local settings, returning the latest
// settings. Can error if can't communicate over http to dimo api. todo: return dbc and device-settings too.
func (vt *vehicleTemplates) GetTemplateSettings(vin string, addr *common.Address) (*loggers.PIDLoggerSettings, error) {
	// read any existing settings
	config, err := vt.lss.ReadPIDsConfig()
	if err != nil {
		vt.logger.Err(err).Msg("could not read settings for templates, continuing")
	}
	var configURLs *gateways.URLConfigResponse
	if len(vin) == 17 {
		configURLs, err = vt.vsd.GetUrlsByVin(vin)
		if err != nil {
			vt.logger.Err(err).Msg("unable to get template urls by vin")
		}
	} else {
		configURLs, err = vt.vsd.GetUrlsByEthAddr(addr)
		if err != nil {
			vt.logger.Err(err).Msg("unable to get template urls by eth addr")
		}
	}
	if err != nil {
		vt.logger.Err(err).Msgf("could not get pids URL for configuration.")
		if config != nil {
			return config, nil
		}
		return nil, err
	}
	// if no change, just return what we have
	if configURLs != nil && config != nil && configURLs.Version == config.Version && configURLs.PidURL == config.PidURL {
		vt.logger.Info().Msgf("vehicle template configuration has not changed, keeping current. version %s", config.Version)
		return config, nil
	}

	pids, err := vt.vsd.GetPIDs(configURLs.PidURL)
	if err != nil {
		vt.logger.Err(err).Msgf("could not get pids template from api")
		if config != nil {
			return config, nil
		}
		return nil, err
	}
	// copy over the response object to the configuration object // possible optimization here to just use same object
	config = &loggers.PIDLoggerSettings{}
	if len(pids.Requests) > 0 {
		for _, item := range pids.Requests {
			config.PIDs = append(config.PIDs, loggers.PIDLoggerItemSettings{
				Formula:  item.Formula,
				Protocol: item.Protocol,
				PID:      item.Pid,
				Mode:     item.Mode,
				Header:   item.Header,
				Interval: item.IntervalSeconds,
				Name:     item.Name,
			})
		}
	}

	err = vt.lss.WritePIDsConfig(*config)
	if err != nil {
		vt.logger.Err(err).Msg("failed to write pids config locally")
	}

	return config, nil
}

// GetTemplateURLsByEth calls gateway to get template url's by eth, with retries. persists to tmp folder by calling logger settings svc if different.
// gets and persists the device settings, pids, and dbc file only if version is different
func (vt *vehicleTemplates) GetTemplateURLsByEth(addr *common.Address) (*loggers.PIDLoggerSettings, error) {
	// todo: need to change the local and external read objects to match better, they should all be saved in different files in tmp
	outdated := false
	localConfig, readLocalErr := vt.lss.ReadPIDsConfig()
	if readLocalErr != nil {
		outdated = true
	}
	configURLs, err := vt.vsd.GetUrlsByEthAddr(addr)
	// todo retries - abstract into gateway, maybe pass in number of retries
	if err != nil {
		// return whatever we had saved locally.
		return localConfig, errors.Wrap(err, "unable to get template url's by eth addr")
	}
	if !outdated && localConfig.Version != configURLs.Version {
		// todo, lss. write template urls

		outdated = true
	}
	if !outdated {
		return localConfig, nil
	}
	pidsConfig, err := vt.vsd.GetPIDs(configURLs.PidURL)
	if err != nil {
		return localConfig, err
	}
	// todo get device settings, dbc
	writeErr := vt.lss.WritePIDsConfig(pidsConfig) // need to use same types on both ends
	if writeErr != nil {
		vt.logger.Err(writeErr).Send()
	}
	return pidsConfig, nil

}
