package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal"
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

func (p *scanVINCmd) Execute(ctx context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	log.Infof("trying to get VIN\n")
	// this is purposely left un-refactored
	vinResp, vinErr := commands.GetVIN(p.unitID)
	if vinErr != nil {
		err := internal.SendErrorPayload(p.unitID, nil, vinErr)
		log.Errorf("failed to send mqtt payload: %s", err.Error())
		log.Panicf("could not get vin %s", vinErr.Error())
	}
	log.Infof("VIN: %s\n", vinResp.VIN)
	log.Infof("Protocol: %s\n", vinResp.Protocol)
	if p.send {
		addr, err := commands.GetEthereumAddress(p.unitID)
		if err != nil {
			// todo retry logic?
			errSend := internal.SendErrorPayload(p.unitID, nil, err)
			log.Errorf("failed to send mqtt payload: %s", errSend.Error())
			log.Panicf("could not get eth address %s", err.Error())
		}

		payload := internal.NewStatusUpdatePayload(p.unitID, addr)
		payload.Data = internal.StatusUpdateData{
			Vin:      vinResp.VIN,
			Protocol: vinResp.Protocol,
		}
		err = internal.SendPayload(&payload, p.unitID)
		if err != nil {
			log.Errorf("failed to send vin over mqtt: %s", err.Error())
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}
