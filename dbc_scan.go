package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/hooks"
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
	return "starts scanning canbus with the passed in dbc file or default on in autopi directory if no parameter"
}
func (*dbcScanCmd) Usage() string {
	return `dbc-scan -file <dbc.file path>`
}

func (p *dbcScanCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.dbcFilePath, "file", "", "dbc file path")
}

func (p *dbcScanCmd) Execute(_ context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	p.logger.Info().Msg("Start Scanning canbus with a DBC file:")
	dbc := loggers.DBCFile
	if p.dbcFilePath != "" {
		dbc = p.dbcFilePath
	}
	fmt.Println("dbc path:" + dbc)

	content, err := os.ReadFile(dbc)
	if err != nil {
		hooks.LogFatal(p.logger, err, "failed to read dbc file")
	}
	d := string(content)
	fmt.Println(d)

	dbcLogger := loggers.NewDBCPassiveLogger(p.logger, &d, "7", nil) // always try, v7 will allow
	ch := make(chan models.SignalData)
	go func() {
		err := dbcLogger.StartScanning(ch)
		if err != nil {
			hooks.LogFatal(p.logger, err, "failed to start scanning")
		}
	}()
	for signal := range ch {
		fmt.Printf("value obtained: %+v \n", signal)
	}
	// if runs ok, this will never hit btw
	return subcommands.ExitSuccess
}
