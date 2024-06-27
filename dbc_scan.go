package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/google/subcommands"
	"github.com/rs/zerolog"
)

type dbcScanCmd struct {
	logger      zerolog.Logger
	dbcFilePath string
}

func (*dbcScanCmd) Name() string { return "dbc-scan" }
func (*dbcScanCmd) Synopsis() string {
	return "starts scanning canbus with the passed in dbc file"
}
func (*dbcScanCmd) Usage() string {
	return `dbc-scan -file <dbc.file path>`
}

func (p *dbcScanCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.dbcFilePath, "file", "", "dbc file path")
}

func (p *dbcScanCmd) Execute(_ context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	p.logger.Info().Msg("Start Scanning canbus with a DBC file:")
	fmt.Println("dbc path:" + p.dbcFilePath)

	content, err := os.ReadFile(p.dbcFilePath)
	if err != nil {
		p.logger.Fatal().Err(err).Send()
	}
	fmt.Println(content)
	d := string(content)

	dbcLogger := loggers.NewDBCPassiveLogger(p.logger, &d, "")
	ch := make(chan models.SignalData)
	go func() {
		err := dbcLogger.StartScanning(ch)
		if err != nil {
			p.logger.Fatal().Err(err).Msg("failed to start scanning")
		}
	}()
	for signal := range ch {
		fmt.Printf("value obtained: %+v \n", signal)
	}
	// if runs ok, this will never hit btw
	return subcommands.ExitSuccess
}
