package main

import (
	"context"
	"flag"

	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type scanJ1939VINCmd struct {
	unitID uuid.UUID
	send   bool
}

func (*scanJ1939VINCmd) Name() string { return "scan-j1939vin" }
func (*scanJ1939VINCmd) Synopsis() string {
	return "scans for j1939 VIN"
}
func (*scanJ1939VINCmd) Usage() string {
	return `scan-j1939vin`
}

func (p *scanJ1939VINCmd) SetFlags(f *flag.FlagSet) {
}

func (p *scanJ1939VINCmd) Execute(_ context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	log.Infof("trying to get j1939 VIN\n")
	// this is purposely left un-refactored
	vl := loggers.NewVINLogger()
	vinResp, err := vl.GetJ1939VIN(p.unitID)
	log.Infof("j1939 VIN: %s\n", vinResp)
	if err != nil {
		log.Errorf("failed to request j1939 VIN: %s", err.Error())
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
