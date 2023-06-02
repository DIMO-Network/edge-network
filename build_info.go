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

func (*buildInfoCmd) Name() string { return "build-info" }
func (*buildInfoCmd) Synopsis() string {
	return "prints out the build info provided by go debug.ReadBuildInfo()"
}
func (*buildInfoCmd) Usage() string {
	return `build-info`
}

// nolint
func (p *buildInfoCmd) SetFlags(_ *flag.FlagSet) {
	// maybe canbus-only option?
}

func (p *buildInfoCmd) Execute(_ context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	log.Infof("printing build info\n\n")

	if info, ok := debug.ReadBuildInfo(); ok {
		log.Printf("Build Info\n\n" + info.String())
	}

	return subcommands.ExitSuccess
}
