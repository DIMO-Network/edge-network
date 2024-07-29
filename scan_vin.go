package main

import (
	"context"
	"flag"

	"github.com/DIMO-Network/edge-network/commands"
	dimoConfig "github.com/DIMO-Network/edge-network/config"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type scanVINCmd struct {
	unitID uuid.UUID
	send   bool
	logger zerolog.Logger
}

func (*scanVINCmd) Name() string { return "scan-vin" }
func (*scanVINCmd) Synopsis() string {
	return "scans for VIN using same command we use for BTE pairing. meant for debugging"
}
func (*scanVINCmd) Usage() string {
	return `scan-vin [-send]`
}

func (p *scanVINCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.send, "send", false, "send result over mqtt to the cloud")
}

func (p *scanVINCmd) Execute(_ context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	p.logger.Info().Msg("trying to get VIN\n")
	// this is purposely left un-refactored
	vl := loggers.NewVINLogger(p.logger)
	addr, err := commands.GetEthereumAddress(p.unitID)
	if err != nil {
		p.logger.Fatal().Msgf("could not get eth address %s", err.Error())
	}
	// read config file
	conf, err := dimoConfig.ReadConfigFromPath("/opt/autopi/config.yaml")
	if err != nil {
		p.logger.Error().Msg("unable to read config file")
		return subcommands.ExitFailure
	}
	ds := network.NewDataSender(p.unitID, *addr, p.logger, models.VehicleInfo{}, *conf)
	vinResp, vinErr := vl.GetVIN(p.unitID, nil)
	if vinErr != nil {
		p.logger.Fatal().Msgf("could not get vin %s", vinErr.Error())
	}
	p.logger.Info().Msgf("VIN: %s\n", vinResp.VIN)
	p.logger.Info().Msgf("Protocol: %s\n", vinResp.Protocol)
	if p.send {
		data := models.FingerprintData{
			Vin:      vinResp.VIN,
			Protocol: vinResp.Protocol,
		}
		err = ds.SendFingerprintData(data)
		if err != nil {
			p.logger.Fatal().Err(err).Msgf("failed to send vin over mqtt: %s", err.Error())
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}
