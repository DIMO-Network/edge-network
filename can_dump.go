package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/config"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"os"
	"strconv"
	"time"
)

// - To scan can bus and save local copy on autopi:
// ./edge-network candump -cycles <cycle_count>  -save

// - To scan can bus and send chunked dump to mqtt:
// ./edge-network candump -cycles <cycle_count> [-send <chunk_size>]

// - To scan can bus and send chunked dump to mqtt AND save local copies on autopi:
// ./edge-network candump -cycles <cycle_count> [-send <chunk_size>] -save

type canDumpCmd struct {
	unitID     uuid.UUID
	save       bool
	cycleCount int
	chunkSize  int
}

func (*canDumpCmd) Name() string { return "candump" }
func (*canDumpCmd) Synopsis() string {
	return "Performs can dump to local file and/or remote file via MQTT"
}
func (*canDumpCmd) Usage() string {
	return `candump [-cycles <int>] [-send <chunkSize_int>] [-save]`
}

func (p *canDumpCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.save, "save", false, "save result to local file")
	f.IntVar(&p.cycleCount, "cycles", 100, "the qty of cycles to record in can dump, default=100")
	f.IntVar(&p.chunkSize, "send", 0, "send result over mqtt to the s3 bucket using <chunk_size>")
}

func (p *canDumpCmd) Execute(_ context.Context, _ *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	log := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Str("version", Version).
		Logger().
		Output(zerolog.ConsoleWriter{Out: os.Stdout})

	println("cycleCount: " + strconv.Itoa(p.cycleCount))
	println("chunkSize: " + strconv.Itoa(p.chunkSize))

	currentTime, _ := time.Now().MarshalJSON()
	currentTime = currentTime[1 : len(currentTime)-1]

	ethAddr, ethErr := commands.GetEthereumAddress(unitID)
	if ethErr != nil {
		log.Err(ethErr).Send()
		return subcommands.ExitFailure
	}

	if p.chunkSize > p.cycleCount {
		log.Error().Msg("chunkSize cannot be greater than cycleCount")
		return subcommands.ExitFailure
	}

	canDumperInstance := new(loggers.PassiveCanDumper)

	canErr := canDumperInstance.ReadCanBus(p.cycleCount, 500000)
	if canErr != nil {
		log.Err(canErr).Send()
		return subcommands.ExitFailure
	}

	// read config file
	conf, ok := args[0].(config.Config)
	if !ok {
		log.Error().Msg("unable to read config file")
		return subcommands.ExitFailure
	}
	if p.chunkSize > 0 && p.save {
		mqttErr := canDumperInstance.WriteToMQTT(log, unitID, *ethAddr, p.chunkSize, string(currentTime), true, conf)
		if mqttErr != nil {
			log.Err(mqttErr).Send()
			return subcommands.ExitFailure
		}
	} else if p.chunkSize > 0 {
		mqttErr := canDumperInstance.WriteToMQTT(log, unitID, *ethAddr, p.chunkSize, string(currentTime), true, conf)
		if mqttErr != nil {
			log.Err(mqttErr).Send()
			return subcommands.ExitFailure
		}
	} else if p.save {
		fileErr := canDumperInstance.WriteToFile("can_dump_" + string(currentTime))
		if fileErr != nil {
			log.Err(fileErr).Send()
			return subcommands.ExitFailure
		}
	}
	return subcommands.ExitSuccess
}
