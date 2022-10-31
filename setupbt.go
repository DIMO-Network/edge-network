package main

import (
	"fmt"
	"os"
	"time"

	"github.com/muka/go-bluetooth/hw"
)

func setupBluez() error {
	btmgmt := hw.NewBtMgmt(adapterID)

	// TODO(elffjs): What's this for?
	if os.Getenv("DOCKER") != "" {
		btmgmt.BinPath = "./bin/docker-btmgmt"
	}

	// Need to turn off the controller to be able to modify the next few settings.
	err := btmgmt.SetPowered(false)
	if err != nil {
		return fmt.Errorf("failed to power off the controller: %w", err)
	}

	err = btmgmt.SetLe(true)
	if err != nil {
		return fmt.Errorf("failed to enable LE: %w", err)
	}

	err = btmgmt.SetBredr(false)
	if err != nil {
		return fmt.Errorf("failed to disable BR/EDR: %w", err)
	}

	time.Sleep(2 * time.Second)

	err = btmgmt.SetPowered(true)
	if err != nil {
		return fmt.Errorf("failed to power on the controller: %w", err)
	}

	err = btmgmt.SetConnectable(true)
	if err != nil {
		return fmt.Errorf("failed to set the controller as connectable: %w", err)
	}

	err = btmgmt.SetBondable(true)
	if err != nil {
		return fmt.Errorf("failed to set the controller as bondable: %w", err)
	}

	err = btmgmt.SetSc(true)
	if err != nil {
		return fmt.Errorf("failed to enable Secure Connections: %w", err)
	}

	err = btmgmt.SetDiscoverable(true)
	if err != nil {
		return fmt.Errorf("failed to set the controller as discoverable: %w", err)
	}

	return nil
}
