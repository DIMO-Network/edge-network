package loggers

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DIMO-Network/edge-network/internal/util"

	"github.com/DIMO-Network/edge-network/internal/models"

	"github.com/pkg/errors"

	"github.com/DIMO-Network/edge-network/internal/loggers/canbus"
	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"
)

//go:generate mockgen -source dbc_passive_logger.go -destination mocks/dbc_passive_logger_mock.go
type DBCPassiveLogger interface {
	StartScanning(ch chan<- models.SignalData) error
	HasDBCFile() bool
	SendCANQuery(header uint32, mode uint32, pid uint32) error
	StopScanning() error
}

type dbcPassiveLogger struct {
	logger  zerolog.Logger
	dbcFile *string
	// found that 5.2 hw did not work with this
	hardwareSupport bool
	pids            []models.PIDRequest
	recv            *canbus.Socket
	send            *canbus.Socket
}

func NewDBCPassiveLogger(logger zerolog.Logger, dbcFile *string, hwVersion string, pids *models.TemplatePIDs) DBCPassiveLogger {
	v, err := strconv.Atoi(hwVersion)
	if err != nil {
		logger.Err(err).Msgf("unable to parse hardware version: %s", hwVersion)
	}
	dpl := &dbcPassiveLogger{logger: logger, dbcFile: dbcFile, hardwareSupport: v >= 7}
	if pids != nil {
		dpl.pids = pids.Requests
	}

	return dpl
}

func (dpl *dbcPassiveLogger) StartScanning(ch chan<- models.SignalData) error {
	// todo switch here for when we add filters only for pid querying but no DBC file
	// note currently this is not possible since we only do native querying and pids when there is a dbc file
	if dpl.dbcFile == nil {
		dpl.logger.Info().Msg("dbcFile is nil - not starting DBC passive logger")
		return nil
	}
	if !dpl.hardwareSupport {
		dpl.logger.Info().Msg("hardware support is not enabled due to old hw - not starting DBC passive logger")
		return nil
	}
	filters, err := dpl.parseDBCHeaders(*dpl.dbcFile)
	if err != nil {
		return errors.Wrapf(err, "failed to parse dbc file: %s", *dpl.dbcFile)
	}

	// manually add 7e8 header filter. We could also iterate over pids and pull out unique response headers
	filters = append(filters, dbcFilter{
		header: 2024, //7e8 // todo handle filters for extended frame
	})

	dpl.recv, err = canbus.New()
	if err != nil {
		return err
	}

	// set hardware filters
	uf := make([]unix.CanFilter, len(filters))
	for i, filter := range filters {
		uf[i].Id = filter.header // wants decimal representation of header - not hex
		uf[i].Mask = unix.CAN_SFF_MASK
	}
	err = dpl.recv.SetFilters(uf)
	if err != nil {
		return fmt.Errorf("cannot set canbus filters: %w", err)
	}
	err = dpl.recv.Bind("can0")
	if err != nil {

		return errors.Wrap(err, "could not bind recv socket")
	}
	// loop
	for {
		frame, err := dpl.recv.Recv()
		if err != nil {
			// improvement - accumulate on this error and if happens too much report up to edge-logs
			dpl.logger.Debug().Err(err).Msg("failed to read frame")
			continue
		}
		//fmt.Printf("%7s  %03x %s\n", recv.Name(), frame.ID, printBytesAsHex(frame.Data)) //debug
		// handle standard PID responses
		if frame.ID == 2024 && len(dpl.pids) > 0 {
			pid := dpl.matchPID(frame)
			if pid != nil {
				dpl.logger.Debug().Msgf("found pid match: %+v", pid)
				floatVal, _, errFormula := ParsePIDBytesWithDBCFormula(frame.Data, pid.Pid, pid.Formula)
				if errFormula != nil {
					dpl.logger.Err(errFormula).Msgf("failed to extract PID data with formula: %s", pid.Formula)
					continue
				}
				dpl.logger.Debug().Msgf("%s value: %f", pid.Name, floatVal)
				// push to channel
				s := models.SignalData{
					Timestamp: time.Now().UnixMilli(),
					Name:      pid.Name,
					Value:     floatVal,
				}
				ch <- s
			} else {
				dpl.logger.Warn().Msgf("did not find pid match for data frame: %s", printBytesAsHex(frame.Data))
			}
			// this frame won't be processed by the DBC filter
			continue
		}
		// handle DBC file - match the frame id to our filters so we can get the right formula
		f := findFilter(filters, frame.ID)
		hexStr := fmt.Sprintf("%02d", frame.Data)
		for _, signal := range f.signals {
			// todo new version of this that just takes in bytes
			floatValue, _, err := ExtractAndDecodeWithDBCFormula(hexStr, "", signal.formula)
			if err != nil {
				dpl.logger.Err(err).Msg("failed to extract float value. hex: " + hexStr)
			}
			s := models.SignalData{
				Timestamp: time.Now().UnixMilli(),
				Name:      signal.signalName,
				Value:     floatValue,
			}
			// push to channel
			ch <- s
		}
	}
}

func (dpl *dbcPassiveLogger) HasDBCFile() bool {
	// todo feel like this should also check for hardware compat
	return dpl.dbcFile != nil && *dpl.dbcFile != ""
}

func (dpl *dbcPassiveLogger) StopScanning() error {
	if dpl.recv != nil {
		errR := dpl.recv.Close()
		if errR != nil {
			return errR
		}
	}
	if dpl.send != nil {
		errS := dpl.send.Close()
		if errS != nil {
			return errS
		}
	}
	return nil
}

// SendCANQuery calls SendCANFrame, just builds up the raw frame with some standards. fire and forget. Responses come in StartScanning filters.
func (dpl *dbcPassiveLogger) SendCANQuery(header uint32, mode uint32, pid uint32) error {
	//02 01 33 00 00 00 00 00 // length mode pid
	// build a hex string and then convert it to byte representation
	pidHex := util.UintToHexStr(pid)
	modeHex := util.UintToHexStr(mode)
	length := "02"
	if len(pidHex) == 4 {
		length = "03"
	}
	payload := length + modeHex + pidHex
	padded := fmt.Sprintf("%-16s", payload)
	paddedWithZeros := strings.Replace(padded, " ", "0", -1)

	data, err := hex.DecodeString(paddedWithZeros)
	if err != nil {
		return errors.Wrap(err, "cannot decode hex string")
	}

	return dpl.SendCANFrame(header, data)
}

// SendCANFrame sends a raw frame on the can bus. Initializes socket if it is nil.
// if the header is bigger that x0fff, sets frame type as extended frame format. Fire and forget.
func (dpl *dbcPassiveLogger) SendCANFrame(header uint32, data []byte) error {
	// init if not set
	if dpl.send == nil {
		var err error
		dpl.send, err = canbus.New()
		if err != nil {
			return errors.Wrap(err, "cannot create canbus socket")
		}
		err = dpl.send.Bind("can0")
		if err != nil {
			return errors.Wrap(err, "cannot bind canbus socket")
		}
	}
	// switch to extended frame if bigger header
	k := canbus.SFF
	if header > 4095 {
		k = canbus.EFF
	}
	_, err := dpl.send.Send(canbus.Frame{
		ID:   header,
		Data: data,
		Kind: k,
	})
	if err != nil {
		return errors.Wrap(err, "cannot send canbus frame")
	}
	return nil
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
	headerSignals := make([]dbcSignal, 0)

	for _, line := range lines {
		var err error

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		// Check if the line starts with "BO_" and if it has at least 2 fields
		if fields[0] == "BO_" && len(fields) >= 2 {
			// we've hit a new header, add all the accumulated signals to a new dbcFilter with the last header
			filters, err = addPrevFilter(header, headerSignals, filters)
			if err != nil {
				return nil, err
			}
			// Extract the header. It is second word in string
			header = fields[1]
			headerSignals = []dbcSignal{} // reset the signals since we're at a new header now
		}

		// Check if the line starts with "SG_" and if it has at least 2 fields, could be multiple SG per header
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

			headerSignals = append(headerSignals, dbcSignal{
				signalName: signalName,
				formula:    formula,
			})
		}
	}
	// check if signals still need to be drained to add them
	filters, err := addPrevFilter(header, headerSignals, filters)
	if err != nil {
		return nil, err
	}

	// Return if no filters were found
	if len(filters) == 0 {
		return nil, fmt.Errorf("no header-formula pairs were found")
	}
	return filters, nil
}

func (dpl *dbcPassiveLogger) matchPID(frame canbus.Frame) *models.PIDRequest {
	for _, pid := range dpl.pids {
		if pid.ResponseHeader == 0 {
			pid.ResponseHeader = 2024 // set the default 7e8 if not set
		}
		if pid.ResponseHeader == frame.ID {
			// todo there can be two byte PIDs in the frame, but need examples of this - is it UDS DID only? No standard OBD2 pids do this
			if pid.Pid == uint32(frame.Data[2]) {
				return &pid
			}
		}
	}
	return &models.PIDRequest{}
}

func addPrevFilter(header string, headerSignals []dbcSignal, filters []dbcFilter) ([]dbcFilter, error) {
	if header != "" && len(headerSignals) > 0 {
		headerUint, err := strconv.ParseUint(header, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("error converting header to uint32: %w", err)
		}
		filter := dbcFilter{
			header:  uint32(headerUint),
			signals: headerSignals,
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

func printBytesAsHex(data []byte) string {
	var blank = strings.Repeat(" ", 24)
	ascii := strings.ToUpper(hex.Dump(data))
	ascii = strings.TrimRight(strings.Replace(ascii, blank, "", -1), "\n")
	return ascii
}

type dbcFilter struct {
	header  uint32
	signals []dbcSignal
}

type dbcSignal struct {
	formula    string
	signalName string
}
