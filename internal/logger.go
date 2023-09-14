package internal

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/internal/gateways"
	"strconv"
	"time"

	"github.com/DIMO-Network/edge-network/internal/api"

	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type LoggerService interface {
	Fingerprint() error
	PIDLoggers(vin string) error
}

type loggerService struct {
	unitID                   uuid.UUID
	vinLog                   loggers.VINLogger
	pidLog                   loggers.PIDLogger
	dataSender               network.DataSender
	loggerSettingsSvc        loggers.LoggerSettingsService
	vehicleSignalDecodingSvc gateways.VehicleSignalDecodingAPIService
}

func NewLoggerService(unitID uuid.UUID, vinLog loggers.VINLogger, pidLog loggers.PIDLogger, dataSender network.DataSender, loggerSettingsSvc loggers.LoggerSettingsService, vehicleSignalDecodingSvc gateways.VehicleSignalDecodingAPIService) LoggerService {
	return &loggerService{unitID: unitID, vinLog: vinLog, pidLog: pidLog, dataSender: dataSender, loggerSettingsSvc: loggerSettingsSvc, vehicleSignalDecodingSvc: vehicleSignalDecodingSvc}
}

const maxFailureAttempts = 5

// Fingerprint checks if ok to start scanning the vehicle and then tries to get the VIN & Protocol via various methods.
// Runs only once when successful.  Checks for saved VIN query from previous run.
func (ls *loggerService) Fingerprint() error {
	// check if ok to start making obd calls etc
	log.Infof("loggers: starting - checking if can start scanning")
	ok, status, err := ls.isOkToScan()
	if err != nil {
		_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "checks to start loggers failed"), &status)
		return errors.Wrap(err, "checks to start loggers failed, no action")
	}
	if !ok {
		e := fmt.Errorf("checks to start loggers failed but no errors reported")
		_ = ls.dataSender.SendErrorPayload(e, &status)
		return e
	}
	log.Infof("loggers: checks passed to start scanning")
	// read any existing settings
	config, err := ls.loggerSettingsSvc.ReadVINConfig()
	if err != nil {
		log.Printf("could not read settings, continuing: %s", err)
	}
	var vqn *string
	if config != nil {
		vqn = &config.VINQueryName
		// check if we do not want to continue scanning VIN for this car - currently determines if we run any loggers (but do note some cars won't respond VIN but yes on most OBD2 stds)
		if config.VINLoggerVersion == loggers.VINLoggerVersion { // if vin logger improves, basically ignore failed attempts as maybe we decoded it.
			if config.VINLoggerFailedAttempts >= maxFailureAttempts {
				if config.VINQueryName != "" {
					// this would be really weird and needs to be addressed
					_ = ls.dataSender.SendErrorPayload(fmt.Errorf("failed attempts exceeded but was previously able to get VIN with query: %s", config.VINQueryName), &status)
				}
				return fmt.Errorf("failed attempts for VIN logger exceeded, not starting loggers")
			}
		}
	}

	// loop over loggers and call them. This needs to be reworked to support more than one thing that is not VIN etc
	for _, logger := range ls.getLoggerConfigs() {
		vinResp, err := logger.ScanFunc(ls.unitID, vqn)
		if err != nil {
			log.WithError(err).Log(log.ErrorLevel)
			// update local settings to increment fail count
			if config == nil {
				config = &loggers.VINLoggerSettings{}
			}
			config.VINLoggerVersion = loggers.VINLoggerVersion
			config.VINLoggerFailedAttempts++
			writeErr := ls.loggerSettingsSvc.WriteVINConfig(*config)
			if writeErr != nil {
				log.WithError(writeErr).Log(log.ErrorLevel)
			}

			_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to get VIN from logger"), &status)
			break
		}
		// save vin query name in settings if not set
		if config == nil || config.VINQueryName == "" {
			config = &loggers.VINLoggerSettings{VINQueryName: vinResp.QueryName, VIN: vinResp.VIN}
			err := ls.loggerSettingsSvc.WriteVINConfig(*config)
			if err != nil {
				log.WithError(err).Log(log.ErrorLevel)
				_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to write logger settings"), &status)
			}
		}

		data := network.FingerprintData{
			Vin:      vinResp.VIN,
			Protocol: vinResp.Protocol,
		}
		data.RpiUptimeSecs = status.Rpi.Uptime.Seconds
		data.BatteryVoltage = status.Spm.Battery.Voltage

		err = ls.dataSender.SendFingerprintData(data)
		if err != nil {
			log.WithError(err).Log(log.ErrorLevel)
		}
	}

	return nil
}

func (ls *loggerService) PIDLoggers(vin string) error {
	// check if ok to start making obd calls etc
	log.Infof("loggers: starting - checking if can start scanning")
	ok, status, err := ls.isOkToScan()
	if err != nil {
		_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "checks to start loggers failed"), &status)
		return errors.Wrap(err, "checks to start loggers failed, no action")
	}
	if !ok {
		e := fmt.Errorf("checks to start loggers failed but no errors reported")
		_ = ls.dataSender.SendErrorPayload(e, &status)
		return e
	}
	log.Infof("loggers: checks passed to start PID loggers")
	// read any existing settings
	config, err := ls.loggerSettingsSvc.ReadPIDsConfig()
	if err != nil {
		log.Printf("could not read settings, continuing: %s", err)
	}

	if config != nil {
		if len(config.PIDs) == 0 {
			pids, err := ls.vehicleSignalDecodingSvc.GetPIDsTemplateByVIN(vin)
			if err != nil {
				log.Printf("could not get pids template from api, continuing: %s", err)
				return err
			}

			config = &loggers.PIDLoggerSettings{}
			if len(pids.Requests) > 0 {
				for _, item := range pids.Requests {
					config.PIDs = append(config.PIDs, loggers.PIDLoggerItemSettings{
						Formula:  item.Formula,
						Protocol: item.Protocol,
						PID:      strconv.FormatInt(item.Pid, 10),
						Mode:     strconv.FormatInt(item.Mode, 10),
						Header:   strconv.FormatInt(item.Header, 10),
						Interval: item.IntervalSeconds,
					})
				}
			}

			err = ls.loggerSettingsSvc.WritePIDsConfig(*config)
			if err != nil {
				log.WithError(err).Log(log.ErrorLevel)
				_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to write pids logger settings"), &status)
			}
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
func (ls *loggerService) isOkToScan() (result bool, status api.PowerStatusResponse, err error) {
	const maxTries = 100
	const voltageMin = 13.2
	tries := 0
	status, httpError := commands.GetPowerStatus(ls.unitID)
	for httpError != nil {
		if tries > maxTries {
			return false, status, fmt.Errorf("loggers: unable to get power.status after %d tries with error %s", maxTries, httpError.Error())
		}
		status, httpError = commands.GetPowerStatus(ls.unitID)
		tries++
		time.Sleep(2 * time.Second)
	}

	log.Infof("loggers: Last Start Reason: %s. Voltage: %f", status.Spm.LastTrigger.Up, status.Spm.Battery.Voltage)
	if status.Spm.LastTrigger.Up == "volt_change" || status.Spm.LastTrigger.Up == "volt_level" || status.Spm.LastTrigger.Up == "stn" {
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
		return false, status, fmt.Errorf("loggers: Spm.LastTrigger.Up value not expected so not starting logger: %s", status.Spm.LastTrigger.Up)
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
