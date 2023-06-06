package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type scanVINCmd struct {
	unitID uuid.UUID
	send   bool
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
	log.Infof("trying to get VIN\n")
	// this is purposely left un-refactored
	vl := loggers.NewVINLogger()
	addr, err := commands.GetEthereumAddress(p.unitID)
	if err != nil {
		log.Panicf("could not get eth address %s", err.Error())
	}
	ds := network.NewDataSender(p.unitID, addr)
	vinResp, vinErr := vl.GetVIN(p.unitID, nil)
	if vinErr != nil {
		log.Panicf("could not get vin %s", vinErr.Error())
	}
	log.Infof("VIN: %s\n", vinResp.VIN)
	log.Infof("Protocol: %s\n", vinResp.Protocol)
	if p.send {
		payload := network.NewStatusUpdatePayload(p.unitID, addr)
		payload.Data = network.StatusUpdateData{
			Vin:      vinResp.VIN,
			Protocol: vinResp.Protocol,
		}
		err = ds.SendPayload(&payload)
		if err != nil {
			log.Errorf("failed to send vin over mqtt: %s", err.Error())
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}
