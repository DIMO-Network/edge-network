package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type scanVINCmd struct {
	unitID uuid.UUID
}

func (*scanVINCmd) Name() string { return "scan-vin" }
func (*scanVINCmd) Synopsis() string {
	return "scans for VIN using same command we use for BTE pairing. meant for debugging"
}
func (*scanVINCmd) Usage() string {
	return `scan-vin`
}

// nolint
func (p *scanVINCmd) SetFlags(f *flag.FlagSet) {
	// maybe canbus-only option?
}

func (p *scanVINCmd) Execute(ctx context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	log.Infof("trying to get VIN\n")
	vin, protocol, err := commands.GetVIN(p.unitID)
	if err != nil {
		log.Panicf("could not get vin %s", err.Error())
	}
	log.Infof("VIN: %s\n", vin)
	log.Infof("Protocol: %s\n", protocol)
	// todo send the vin

	return subcommands.ExitSuccess
}
