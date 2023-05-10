package main

import (
	"context"
	"flag"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/subcommands"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"time"
)

type scanVINCmd struct {
	unitID uuid.UUID
	send   bool
}

func (*scanVINCmd) Name() string { return "scan-vin" }
func (*scanVINCmd) Synopsis() string {
	return "scans for VIN using same command we use for BTE pairing. meant for debugging"
}
func (*scanVINCmd) Usage() string {
	return `scan-vin [-send]`
}

func (p *scanVINCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&p.send, "send", false, "send result over mqtt to the cloud")
}

func (p *scanVINCmd) Execute(ctx context.Context, _ *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	log.Infof("trying to get VIN\n")
	vin, protocol, err := commands.GetVIN(p.unitID)
	if err != nil {
		log.Panicf("could not get vin %s", err.Error())
	}
	log.Infof("VIN: %s\n", vin)
	log.Infof("Protocol: %s\n", protocol)
	if p.send {
		addr, err := commands.GetEthereumAddress(p.unitID)
		if err != nil {
			log.Panicf("could not get eth address %s", err.Error())
		}

		err = sendStatusVIN(vin, protocol, addr, p.unitID)
		if err != nil {
			log.Errorf("failed to send vin over mqtt: %s", err.Error())
		}
	}

	return subcommands.ExitSuccess
}

func sendStatusVIN(vin, protocol string, ethAddress common.Address, autopiUnitID uuid.UUID) error {
	payload := internal.StatusUpdatePayload{
		Subject:         autopiUnitID.String(),
		EthereumAddress: ethAddress.Hex(),
		UnitID:          autopiUnitID.String(),
		Timestamp:       time.Now().UTC().UnixMilli(),
		Data: internal.StatusUpdateData{
			Vin:      vin,
			Protocol: protocol,
		},
	}
	err := internal.SendPayload(&payload, autopiUnitID)
	if err != nil {
		return err
	}
	return nil
}
