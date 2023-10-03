package internal

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/internal/gateways"

	"github.com/DIMO-Network/edge-network/internal/api"

	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type FingerprintRunner interface {
	Fingerprint() error
}

type fingerprintRunner struct {
	unitID                   uuid.UUID
	vinLog                   loggers.VINLogger
	pidLog                   loggers.PIDLogger
	dataSender               network.DataSender
	loggerSettingsSvc        loggers.LoggerSettingsService
	vehicleSignalDecodingSvc gateways.VehicleSignalDecodingAPIService
	logger                   zerolog.Logger
}

func NewFingerprintRunner(unitID uuid.UUID, vinLog loggers.VINLogger, pidLog loggers.PIDLogger, dataSender network.DataSender, loggerSettingsSvc loggers.LoggerSettingsService, vehicleSignalDecodingSvc gateways.VehicleSignalDecodingAPIService, logger zerolog.Logger) FingerprintRunner {
	return &fingerprintRunner{unitID: unitID, vinLog: vinLog, pidLog: pidLog, dataSender: dataSender, loggerSettingsSvc: loggerSettingsSvc, vehicleSignalDecodingSvc: vehicleSignalDecodingSvc, logger: logger}
}

const maxFailureAttempts = 5

// Fingerprint checks if ok to start scanning the vehicle and then tries to get the VIN & Protocol via various methods.
// Runs only once when successful.  Checks for saved VIN query from previous run.
func (ls *fingerprintRunner) Fingerprint() error {
	// check if ok to start making obd calls etc
	ls.logger.Info().Msg("loggers: starting - checking if can start scanning")
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
	ls.logger.Info().Msg("loggers: checks passed to start scanning")
	// read any existing settings
	config, err := ls.loggerSettingsSvc.ReadVINConfig()
	if err != nil {
		ls.logger.Info().Msgf("could not read settings, continuing: %s", err)
	}
	var vqn *string
	if config != nil {
		vqn = &config.VINQueryName
		// check if we do not want to continue scanning VIN for this car - currently determines if we run any loggers (but do note some cars won't respond VIN but yes on most OBD2 stds)
		if config.VINLoggerVersion == loggers.VINLoggerVersion { // if vin vinLogger improves, basically ignore failed attempts as maybe we decoded it.
			if config.VINLoggerFailedAttempts >= maxFailureAttempts {
				if config.VINQueryName != "" {
					// this would be really weird and needs to be addressed
					_ = ls.dataSender.SendErrorPayload(fmt.Errorf("failed attempts exceeded but was previously able to get VIN with query: %s", config.VINQueryName), &status)
				}
				return fmt.Errorf("failed attempts for VIN vinLogger exceeded, not starting loggers")
			}
		}
	}
	// scan for VIN
	vinLogger := LoggerProperties{
		SignalName: "vin",
		Interval:   0,
		ScanFunc:   ls.vinLog.GetVIN,
	}
	vinResp, err := vinLogger.ScanFunc(ls.unitID, vqn)
	if err != nil {
		if config == nil {
			config = &loggers.VINLoggerSettings{}
		}
		ls.logger.Err(err).Msgf("failed to scan for vin. fail count: %d", config.VINLoggerFailedAttempts)
		// update local settings to increment fail count
		config.VINLoggerVersion = loggers.VINLoggerVersion
		config.VINLoggerFailedAttempts++
		writeErr := ls.loggerSettingsSvc.WriteVINConfig(*config)
		if writeErr != nil {
			ls.logger.Err(writeErr).Send()
		}

		_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to get VIN from vinLogger"), &status)
	}
	// save vin query name in settings if not set
	if config == nil || config.VINQueryName == "" {
		config = &loggers.VINLoggerSettings{VINQueryName: vinResp.QueryName, VIN: vinResp.VIN}
		err := ls.loggerSettingsSvc.WriteVINConfig(*config)
		if err != nil {
			ls.logger.Err(err).Send()
			_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to write vinLogger settings"), &status)
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
		ls.logger.Err(err).Send()
	}

	return nil
}

// once logger starts, for signals we want to grab continuosly eg odometer, we need more sophisticated logger pausing: from shaolin-
//Yeah so the device itself will still try and talk to the canbus with PIDS when the vehicle is off and then these german cars see this and think someone is trying to break into the car so it sets off the car alarm
//So what we did is we disabled the obd manager so it doesnt ask for PIDs anymore so you dont get obd data
//so what this solution does, is while the car is on and in motion, the obd manager is enabled, will send PIDS and grab data, then once the vehicle stops, the manager turns off
//its a more advanced obd logger pausing

// isOkToScan checks if the power status and other heuristics to determine if ok to start Open CAN scanning and PID requests. Blocking.
func (ls *fingerprintRunner) isOkToScan() (result bool, status api.PowerStatusResponse, err error) {
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

	ls.logger.Info().Msgf("loggers: Last Start Reason: %s. Voltage: %f", status.Spm.LastTrigger.Up, status.Spm.Battery.Voltage)
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

type LoggerProperties struct {
	// SignalName name of signal to be published over mqtt
	SignalName string
	// Interval is how often to run. 0 means only on start
	Interval int32
	// Function to call to get the data from the vehicle
	ScanFunc func(uuid2 uuid.UUID, qn *string) (*loggers.VINResponse, error)
}