package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

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
	//f.BoolVar(&p.send, "send", false, "send result over mqtt to the s3 bucket")
	f.BoolVar(&p.save, "save", false, "save result to local file")
	f.IntVar(&p.cycleCount, "cycles", 100, "the qty of cycles to record in can dump, default=100")
	f.IntVar(&p.chunkSize, "send", 0, "send result over mqtt to the s3 bucket using <chunk_size>")
}

func (p *canDumpCmd) Execute(_ context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	println("cycleCount: " + strconv.Itoa(p.cycleCount))
	println("chunkSize: " + strconv.Itoa(p.chunkSize))

	currentTime, _ := time.Now().MarshalJSON()
	currentTime = currentTime[1 : len(currentTime)-1]

	ethAddr, ethErr := commands.GetEthereumAddress(unitID)
	if ethErr != nil {
		println(ethErr.Error())
		log.Errorf(ethErr.Error())
		return subcommands.ExitFailure
	}

	if p.chunkSize > p.cycleCount {
		println("chunkSize cannot be greater than cycleCount")
		log.Errorf("chunkSize cannot be greater than cycleCount")
		//println(chunkSizeErr.Error())
		return subcommands.ExitFailure
	}

	canDumperInstance := new(loggers.PassiveCanDumper)

	canErr := canDumperInstance.ReadCanBus(p.cycleCount, 500000)
	if canErr != nil {
		println(canErr.Error())
		log.Errorf(canErr.Error())
		return subcommands.ExitFailure
	}

	if p.chunkSize > 0 && p.save {
		mqttErr := canDumperInstance.WriteToMQTT(unitID, *ethAddr, p.chunkSize, string(currentTime), true)
		if mqttErr != nil {
			println(mqttErr.Error())
			log.Errorf(mqttErr.Error())
			return subcommands.ExitFailure
		}
	} else if p.chunkSize > 0 {
		mqttErr := canDumperInstance.WriteToMQTT(unitID, *ethAddr, p.chunkSize, string(currentTime), false)
		if mqttErr != nil {
			println(mqttErr.Error())
			log.Errorf(mqttErr.Error())
			return subcommands.ExitFailure
		}
	} else if p.save {
		fileErr := canDumperInstance.WriteToFile("can_dump_" + string(currentTime))
		if fileErr != nil {
			println(fileErr.Error())
			log.Errorf(fileErr.Error())
			return subcommands.ExitFailure
		}
	}
	return subcommands.ExitSuccess
}
