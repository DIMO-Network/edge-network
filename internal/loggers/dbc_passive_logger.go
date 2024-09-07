package loggers

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DIMO-Network/edge-network/internal/hooks"

	"github.com/DIMO-Network/edge-network/internal/util"

	"github.com/DIMO-Network/edge-network/internal/models"

	"github.com/pkg/errors"

	"github.com/DIMO-Network/edge-network/internal/canbus"
	"github.com/rs/zerolog"
	"golang.org/x/sys/unix"
)

//go:generate mockgen -source dbc_passive_logger.go -destination mocks/dbc_passive_logger_mock.go
type DBCPassiveLogger interface {
	StartScanning(ch chan<- models.SignalData) error
	// UseNativeScanLogger uses a variety of logic to decide if we should enable DBC file support as well as native scanning (they go hand in hand)
	UseNativeScanLogger() bool
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
	// used for filtering for PIDs
	pidRespHdrs map[uint32]struct{}
}

func NewDBCPassiveLogger(logger zerolog.Logger, dbcFile *string, hwVersion string, pids *models.TemplatePIDs) DBCPassiveLogger {
	v, err := strconv.Atoi(hwVersion)
	if err != nil {
		logger.Err(err).Msgf("unable to parse hardware version: %s", hwVersion)
	}
	dpl := &dbcPassiveLogger{logger: logger, dbcFile: dbcFile, hardwareSupport: v >= 6} // have only tested in 7+ working, for sure 5.2 nogo
	if pids != nil {
		dpl.pids = pids.Requests
		dpl.pidRespHdrs = getUniqueResponseHeaders(dpl.pids)
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
	// add any PID or DID filters
	for rh := range dpl.pidRespHdrs {
		filters = append(filters, dbcFilter{
			header: rh,
		})
	}

	dpl.recv, err = canbus.New()
	if err != nil {
		return err
	}

	// set hardware filters
	uf := make([]unix.CanFilter, len(filters))
	for i, filter := range filters {
		uf[i].Id = filter.header // wants decimal representation of header - not hex
		if filter.header > 4095 {
			uf[i].Mask = unix.CAN_EFF_MASK // extended frame
		} else {
			uf[i].Mask = unix.CAN_SFF_MASK // standard frame
		}
	}
	err = dpl.recv.SetFilters(uf)
	if err != nil {
		return fmt.Errorf("cannot set canbus filters: %w", err)
	}
	err = dpl.recv.Bind("can0")
	if err != nil {

		return errors.Wrap(err, "could not bind recv socket")
	}
	// loop for each frame
	for {
		frame, err := dpl.recv.Recv()
		if err != nil {
			// todo improvement- dmytro - accumulate on this error and if happens too much report up to edge-logs
			dpl.logger.Debug().Err(err).Msg("failed to read frame")
			continue
		}

		// handle standard PID responses
		if _, ok := dpl.pidRespHdrs[frame.ID]; ok {
			pid := dpl.matchPID(frame)
			if pid != nil {
				dpl.logger.Debug().Msgf("found pid match: %+v", pid)
				floatVal, _, errFormula := ParsePIDBytesWithDBCFormula(frame.Data, pid.Pid, pid.Formula)
				if errFormula != nil {
					dpl.logger.Err(errFormula).Ctx(context.WithValue(context.Background(), hooks.LogToMqtt, "true")).
						Msgf("failed to extract PID data with formula: %s, resp data: %s, name: %s",
							pid.Formula, printBytesAsHex(frame.Data), pid.Name)
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
				msg := fmt.Sprintf("did not find pid match for data frame: %s", printBytesAsHex(frame.Data))
				hooks.LogWarn(dpl.logger, msg, hooks.WithStopLogAfter(2))
			}
			// this frame won't be processed by the DBC filter
			continue
		}
		// todo can we get a test around this?
		// handle DBC file - match the frame id to our filters so we can get the right formula
		f := findFilter(filters, frame.ID)
		hexStr := fmt.Sprintf("%02d", frame.Data)
		for _, signal := range f.signals {
			floatValue, _, err := DecodePassiveFrame(frame.Data, signal.formula)
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

// UseNativeScanLogger decide if should enable native scanning / querying based on: dbc file existing and hardware support for our impl
func (dpl *dbcPassiveLogger) UseNativeScanLogger() bool {
	// in next revision this could just be on hw support and if template pids are supported
	return dpl.hardwareSupport && dpl.dbcFile != nil && *dpl.dbcFile != ""
}

func (dpl *dbcPassiveLogger) StopScanning() error {
	if dpl.recv != nil {
		errR := dpl.recv.Close()
		if errR != nil {
			return errR
		}
	}
	return nil
}

// getUniqueResponseHeaders returns only the unique response headers in the pids
func getUniqueResponseHeaders(pids []models.PIDRequest) map[uint32]struct{} {
	hdrs := make(map[uint32]struct{})
	for _, pid := range pids {
		hdrs[pid.ResponseHeader] = struct{}{}
	}
	return hdrs
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
		return errors.Wrapf(err, "cannot decode hex string: %s", paddedWithZeros)
	}

	return dpl.SendCANFrame(header, data)
}

// SendCANFrame sends a raw frame on the can bus. Initializes socket if it is nil.
// if the header is bigger that x0fff, sets frame type as extended frame format. Fire and forget.
func (dpl *dbcPassiveLogger) SendCANFrame(header uint32, data []byte) error {
	send, err := canbus.New()
	if err != nil {
		return errors.Wrap(err, "cannot create canbus socket")
	}
	defer send.Close()
	err = send.Bind("can0")
	if err != nil {
		return errors.Wrap(err, "cannot bind canbus socket")
	}

	// switch to extended frame if bigger header
	k := canbus.SFF
	if header > 4095 {
		k = canbus.EFF
	}
	_, err = send.Send(canbus.Frame{
		ID:   header,
		Data: data,
		Kind: k,
	})
	if err != nil {
		return errors.Wrapf(err, "cannot send canbus frame: hdr %d data: %s kind: %s", header, printBytesAsHex(data), k)
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
		if pid.ResponseHeader == frame.ID {
			// todo UDS there can be two byte PIDs in the frame, but need examples of this - is it UDS DID only? No standard OBD2 pids do this
			if pid.Pid == uint32(frame.Data[2]) {
				return &pid
			}
		}
	}
	return nil
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
