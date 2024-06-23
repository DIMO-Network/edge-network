package loggers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/DIMO-Network/edge-network/internal/loggers/canbus"
	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"
)

//go:generate mockgen -source dbc_passive_logger.go -destination mocks/dbc_passive_logger_mock.go
type DBCPassiveLogger interface {
	StartScanning(dbcFile string) error // todo: do we return a channel or have a channel passed in to communicate updates to?
}

type dbcPassiveLogger struct {
	logger zerolog.Logger
}

func NewDBCPassiveLogger(logger zerolog.Logger) DBCPassiveLogger {
	return &dbcPassiveLogger{logger: logger}
}

func (dpl *dbcPassiveLogger) StartScanning(dbcFile string) error {
	filters, err := dpl.parseDBCHeaders(dbcFile)
	if err != nil {
		return errors.Wrapf(err, "failed to pase dbc file: %s", dbcFile)
	}

	recv, err := canbus.New()
	if err != nil {
		return err
	}
	defer recv.Close()

	// set hardware filters
	uf := make([]unix.CanFilter, len(filters))
	for i, filter := range filters {
		uf[i].Id = filter.header // wants decimal representation of header - not hex
		uf[i].Mask = unix.CAN_SFF_MASK
	}
	err = recv.SetFilters(uf)
	if err != nil {
		return fmt.Errorf("cannot set canbus filters: %w", err)
	}
	err = recv.Bind("can0")
	if err != nil {

		return errors.Wrap(err, "could not bind recv socket")
	}

	// loop
	for {
		frame, err := recv.Recv()
		if err != nil {
			dpl.logger.Err(err).Msg("failed to read frame")
			continue
		}
		fmt.Printf("frame-%02d: (id=0x%x)\n", frame.Data, frame.ID)
		// match the frame id to our filters so we can get the right formula
		f := findFilter(filters, frame.ID)
		hexStr := fmt.Sprintf("%02d", frame.Data)
		// todo will this work with empty pid? do we need new decoder?
		floatValue, _, err := ExtractAndDecodeWithDBCFormula(hexStr, "", f.formula)
		if err != nil {
			dpl.logger.Err(err).Msg("failed to extract float value. hex: " + hexStr)
		}
		fmt.Printf("value obtained: %f\n", floatValue)
		// todo how can i send this to a channel

	}
}

func findFilter(filters []dbcFilter, id uint32) *dbcFilter {
	for i := range filters {
		if filters[i].header == id {
			return &filters[i]
		}
	}
	return nil
}

func (dpl *dbcPassiveLogger) parseDBCHeaders(dbcFile string) ([]dbcFilter, error) {
	var filters []dbcFilter
	lines := strings.Split(dbcFile, "\n")

	var header string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		// Check if the line starts with "BO_" and if it has at least 2 fields
		if fields[0] == "BO_" && len(fields) >= 2 {
			// Extract the header. It is second word in string
			header = fields[1]
		}
		// Check if the line starts with "SG_" and if it has at least 2 fields
		if fields[0] == "SG_" && len(fields) >= 2 {
			// Extract the formula, which is after the "SG_" keyword.
			// We join the rest of the line to capture multi-word formulas too
			sg := strings.Join(fields[1:], " ")
			splitSg := strings.Split(sg, ":")
			if len(splitSg) != 2 {
				return nil, fmt.Errorf("invalid sg format: %s", sg)
			}
			signalName := strings.TrimSpace(splitSg[0])
			formula := strings.TrimSpace(splitSg[1])
			headerUint, err := strconv.ParseUint(header, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("error converting header to uint32: %w", err)
			}
			filters = append(filters, dbcFilter{header: uint32(headerUint), formula: formula, signalName: signalName})
			// Reset header for next header-formula pair
			header = ""
		}
	}
	// Return if no filters were found
	if len(filters) == 0 {
		return nil, fmt.Errorf("no header-formula pairs were found")
	}
	return filters, nil
}

type dbcFilter struct {
	header     uint32
	formula    string
	signalName string
}
