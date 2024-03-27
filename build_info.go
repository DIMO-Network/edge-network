package main

import (
	"context"
	"flag"
	"github.com/rs/zerolog"
	"runtime/debug"

	"github.com/google/subcommands"
)

type buildInfoCmd struct {
	logger zerolog.Logger
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
	p.logger.Info().Msg("printing build info\n\n")

	if info, ok := debug.ReadBuildInfo(); ok {
		p.logger.Info().Msgf("Build Info\n\n" + info.String())
	}

	return subcommands.ExitSuccess
}
