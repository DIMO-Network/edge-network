package main

import (
	"context"
	"flag"
	"github.com/google/subcommands"
	log "github.com/sirupsen/logrus"
	"runtime/debug"
)

type buildInfoCmd struct {
}

func (*buildInfoCmd) Name() string { return "scan-vin" }
func (*buildInfoCmd) Synopsis() string {
	return "scans for VIN using same command we use for BTE pairing. meant for debugging"
}
func (*buildInfoCmd) Usage() string {
	return `scan-vin`
}

// nolint
func (p *buildInfoCmd) SetFlags(f *flag.FlagSet) {
	// maybe canbus-only option?
}

func (p *buildInfoCmd) Execute(ctx context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	log.Infof("printing build info\n\n")

	if info, ok := debug.ReadBuildInfo(); ok {
		log.Printf("Build Info\n\n" + info.String())
	}

	return subcommands.ExitSuccess
}
