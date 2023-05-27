package internal

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"
	"time"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type LoggerService interface {
	StartLoggers() error
}

type loggerService struct {
	unitID     uuid.UUID
	vinLog     loggers.VINLogger
	dataSender network.DataSender
}

func NewLoggerService(unitID uuid.UUID, vinLog loggers.VINLogger, dataSender network.DataSender) LoggerService {
	return &loggerService{unitID: unitID, vinLog: vinLog, dataSender: dataSender}
}

// StartLoggers checks if ok to start scanning the vehicle and then according to configuration scans and sends data periodically
func (ls *loggerService) StartLoggers() error {
	// check if ok to start making obd calls etc
	log.Infof("loggers: starting - checking if can start scanning")
	ok, err := ls.isOkToScan()
	if err != nil {
		return errors.Wrap(err, "checks to start loggers failed, no action")
	}
	if !ok {
		return fmt.Errorf("checks to start loggers failed but no errors reported")
	}
	log.Infof("loggers: checks passed to start scanning")
	ethAddr, err := commands.GetEthereumAddress(ls.unitID)
	if err != nil {
		log.WithError(err).Log(log.ErrorLevel)
		_ = ls.dataSender.SendErrorPayload(ls.unitID, ethAddr, err)
	}
	// loop over loggers and call them. This needs to be reworked to support more than one thing that is not VIN etc
	for _, logger := range ls.getLoggerConfigs() {
		vinResp, err := logger.ScanFunc(ls.unitID, nil) // todo: check if queryname is stored and pass in
		if err != nil {
			log.WithError(err).Log(log.ErrorLevel)
			_ = ls.dataSender.SendErrorPayload(ls.unitID, ethAddr, err)
			break
		}
		p := network.NewStatusUpdatePayload(ls.unitID, ethAddr)
		p.Data = network.StatusUpdateData{
			Vin:      vinResp.VIN,
			Protocol: vinResp.Protocol,
		}
		err = ls.dataSender.SendPayload(&p, ls.unitID)
		if err != nil {
			log.WithError(err).Log(log.ErrorLevel)
		}
	}

	return nil
}

// once logger starts, for signals we want to grab continuosly eg odometer, we need more sophisticated logger pausing: from shaolin-
//Yeah so the device itself will still try and talk to the canbus with PIDS when the vehicle is off and then these german cars see this and think someone is trying to break into the car so it sets off the car alarm
//So what we did is we disabled the obd manager so it doesnt ask for PIDs anymore so you dont get obd data
//so what this solution does, is while the car is on and in motion, the obd manager is enabled, will send PIDS and grab data, then once the vehicle stops, the manager turns off
//its a more advanced obd logger pausing

// isOkToScan checks if the power status and other heuristics to determine if ok to start Open CAN scanning and PID requests. Blocking.
func (ls *loggerService) isOkToScan() (result bool, err error) {
	const maxTries = 100
	const voltageMin = 13.2
	tries := 0
	status, httpError := commands.GetPowerStatus(ls.unitID)
	for httpError != nil {
		if tries > maxTries {
			return false, fmt.Errorf("loggers: unable to get power.status after %d tries with error %s", maxTries, httpError.Error())
		}
		status, httpError = commands.GetPowerStatus(ls.unitID)
		tries++
		time.Sleep(2 * time.Second)
	}

	log.Printf("loggers: Last Start Reason: %s. Voltage: %f", status.Spm.LastTrigger.Up, status.Spm.Battery.Voltage)
	if status.Spm.LastTrigger.Up == "volt_change" || status.Spm.LastTrigger.Up == "volt_level" {
		if status.Spm.Battery.Voltage >= voltageMin {
			// good to start scanning
			result = true
			return
		}
		// loop a few more times
		tries = 0
		for status.Spm.Battery.Voltage < voltageMin {
			if tries > maxTries {
				err = fmt.Errorf("loggers: did not reach a satisfactory voltage to start loggers: %f", status.Spm.Battery.Voltage)
				break
			}
			status, httpError = commands.GetPowerStatus(ls.unitID)
			tries++
			time.Sleep(2 * time.Second)
		}
		if httpError != nil {
			err = httpError
		}
	} else {
		return false, fmt.Errorf("loggers: Spm.LastTrigger.Up value not expected so not starting logger")
	}
	// this may be an initial pair or something else so we don't wanna start loggers, just exit
	result = false
	return
}

func (ls *loggerService) getLoggerConfigs() []LoggerProperties {

	return []LoggerProperties{
		{
			SignalName: "vin",
			Interval:   0,
			ScanFunc:   ls.vinLog.GetVIN,
		},
	}
}

type LoggerProperties struct {
	// SignalName name of signal to be published over mqtt
	SignalName string
	// Interval is how often to run. 0 means only on start
	Interval int32
	// Function to call to get the data from the vehicle
	ScanFunc func(uuid2 uuid.UUID, qn *string) (*loggers.VINResponse, error)
}
