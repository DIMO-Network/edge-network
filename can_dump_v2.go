package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/internal/canbus"
	"github.com/google/subcommands"
	"golang.org/x/sys/unix"
)

// note that the cloud console does not have a continuous output, so if using that consider also using the cycles option

type canDumpV2Cmd struct {
	headerFilter uint
	logger       zerolog.Logger
	cycleCount   int
}

func (*canDumpV2Cmd) Name() string { return "can-dump-v2" }
func (*canDumpV2Cmd) Synopsis() string {
	return "can-dump prints data flowing on CAN bus, with optional filters"
}
func (*canDumpV2Cmd) Usage() string {
	return `can-dump-v2 [-header <uint>] [-cycles <int>]`
}

func (p *canDumpV2Cmd) SetFlags(f *flag.FlagSet) {
	f.UintVar(&p.headerFilter, "header", 0, "optional header filter in numeric form eg. 7e8 would be 2024")
	f.IntVar(&p.cycleCount, "cycles", 0, "the qty of cycles to record in can dump. Useful when running from cloud console")
}

func (p *canDumpV2Cmd) Execute(_ context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	// print something about starting
	sck, err := canbus.New()
	if err != nil {
		p.logger.Fatal().Err(err).Msg("failed to connect to CAN")
	}
	defer sck.Close()

	if p.headerFilter > 0 {
		fmt.Printf("setting filter to %d\n", p.headerFilter)

		uf := make([]unix.CanFilter, 1)
		uf[0].Id = uint32(p.headerFilter)
		if p.headerFilter > 4095 {
			uf[0].Mask = unix.CAN_EFF_MASK // extended frame
		} else {
			uf[0].Mask = unix.CAN_SFF_MASK // standard frame
		}
		err = sck.SetFilters(uf)
		if err != nil {
			p.logger.Fatal().Err(err).Msg("failed to set filters")
		}
	}

	err = sck.Bind("can0")
	if err != nil {
		p.logger.Fatal().Err(err).Msg("failed to bind can0")
	}

	var blank = strings.Repeat(" ", 24)

	if p.cycleCount == 0 {
		p.cycleCount = 9999 // if nothing set let's just have a high number
	}

	for i := 0; i < p.cycleCount; i++ {
		msg, err := sck.Recv()
		if err != nil {
			p.logger.Fatal().Err(err).Msg("failed to recv")
		}
		ascii := strings.ToUpper(hex.Dump(msg.Data))
		ascii = strings.TrimRight(strings.Replace(ascii, blank, "", -1), "\n")
		fmt.Printf("%7s  %03x %s\n", sck.Name(), msg.ID, ascii)
	}

	return subcommands.ExitSuccess
}
