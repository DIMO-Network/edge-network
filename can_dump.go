package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"time"
)

type canDumpCmd struct {
	unitID     uuid.UUID
	send       bool
	save       bool
	cycleCount int
	chunkSize  int
}

func (*canDumpCmd) Name() string { return "candump" }
func (*canDumpCmd) Synopsis() string {
	return "Performs can dump to local file and/or remote file via MQTT"
}
func (*canDumpCmd) Usage() string {
	return `candump cycleCount=<int> [chunkSize=<int>] [-send] [-save]`
}

func (p *canDumpCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.send, "send", false, "send result over mqtt to the s3 bucket")
	f.BoolVar(&p.save, "save", false, "save result to local file")
	f.IntVar(&p.cycleCount, "cycleCount", 0, "the qty of cycles to perform can dump")
	f.IntVar(&p.chunkSize, "chunkSize", 0, "the max qty of frames to send in a single MQTT message")
}

func (p *canDumpCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {

	/*cycleCount, chunkSize := 0, 0
	var cycleCountErr, chunkSizeErr error
	for i, arg := range f.Args() {
		if i == 1 {
			cycleCount, cycleCountErr = strconv.Atoi(arg)
			if cycleCountErr != nil {
				println("unable to read cycleCount value from command")
				println(cycleCountErr.Error())
				os.Exit(1)
			}
		} else if i == 2 {
			chunkSize, chunkSizeErr = strconv.Atoi(arg)
		} else if i > 2 {
			break
		}

	}*/

	currentTime, _ := time.Now().MarshalJSON()
	currentTime = currentTime[1 : len(currentTime)-1]

	ethAddr, ethErr := commands.GetEthereumAddress(unitID)
	if ethErr != nil {
		println(ethErr.Error())
		log.Errorf(ethErr.Error())
		return subcommands.ExitFailure
	}

	if p.send {
		if p.cycleCount == 0 /*|| cycleCountErr != nil */ {
			println("unable to read cycleCount value from command")
			log.Errorf("unable to read cycleCount value from command")
			//println(cycleCountErr.Error())
			return subcommands.ExitFailure
		}
		if p.chunkSize == 0 /*|| chunkSizeErr != nil*/ {
			println("unable to read chunkSize value from command")
			log.Errorf("unable to read chunkSize value from command")
			//println(chunkSizeErr.Error())
			return subcommands.ExitFailure
		}
	}
	if p.save {
		if p.cycleCount == 0 {
			println("unable to read cycleCount value from command")
			log.Errorf("unable to read cycleCount value from command")
			return subcommands.ExitFailure
		}
	}
	canDumperInstance := new(loggers.PassiveCanDumper)

	canErr := canDumperInstance.ReadCanBus(p.cycleCount, 500000)
	if canErr != nil {
		println(canErr.Error())
		log.Errorf(canErr.Error())
		return subcommands.ExitFailure
	}

	if p.send && p.save {
		mqttErr := canDumperInstance.WriteToMQTT(unitID, *ethAddr, p.chunkSize, string(currentTime), true)
		if mqttErr != nil {
			println(mqttErr.Error())
			log.Errorf(mqttErr.Error())
			return subcommands.ExitFailure
		}
	} else if p.send {
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
