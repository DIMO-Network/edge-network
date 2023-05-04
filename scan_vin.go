package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"time"
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
	vin, protocol, err := commands.GetVIN(p.unitID)
	if err != nil {
		log.Panicf("could not get vin %s", err.Error())
	}
	log.Infof("VIN: %s\n", vin)
	log.Infof("Protocol: %s\n", protocol)
	if p.send {
		sendStatusVIN(vin, protocol, p.unitID.String())
	}

	return subcommands.ExitSuccess
}

func sendStatusVIN(vin, protocol, autopiUnitID string) error {
	payload := internal.StatusUpdatePayload{
		Subject: autopiUnitID,
		Data: internal.StatusUpdateData{
			Device: internal.StatusUpdateDevice{
				Timestamp: time.Now().UnixMilli(),
				UnitID:    autopiUnitID,
			},
			VinTest:      vin,
			ProtocolTest: protocol,
			Signals: internal.StatusUpdateSignals{
				VinTest:      internal.StringSignal{Value: vin},
				ProtocolTest: internal.StringSignal{Value: protocol},
			},
		},
	}
	err := internal.SendPayload(&payload)
	if err != nil {
		return err
	}
	return nil
}
