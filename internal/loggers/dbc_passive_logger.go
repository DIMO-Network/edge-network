package loggers

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/loggers/canbus"
	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"
)

//go:generate mockgen -source dbc_passive_logger.go -destination mocks/dbc_passive_logger_mock.go
type DBCPassiveLogger interface {
	StartScanning(dbcFile string) // todo: do we return a channel or have a channel passed in to communicate updates to?
}

type dbcPassiveLogger struct {
	logger zerolog.Logger
}

func (dpl *dbcPassiveLogger) StartScanning(dbcFile string) error {
	filters := dpl.parseDBCHeaders(dbcFile)
	if len(filters) == 0 {
		return fmt.Errorf("no dbc headers found in %s", dbcFile)
	}

	recv1, err := canbus.New()
	if err != nil {
		return err
	}
	defer recv1.Close()
	// todo loop
	err = recv1.SetFilters([]unix.CanFilter{
		// set the id's based on uint representation of hex, although DBC file i think is already in numeric form
		{Id: filters[0].header, Mask: unix.CAN_SFF_MASK},
	})
	if err != nil {
		return fmt.Errorf("cannot set canbus filters: %w", err)
	}

	return nil
}

// todo implement, test
func (dpl *dbcPassiveLogger) parseDBCHeaders(string) []dbcFilter {}

type dbcFilter struct {
	header  uint32
	formula string
	pid     string
}
