package internal

import (
	"fmt"
	"time"

	"github.com/DIMO-Network/edge-network/internal/models"

	"github.com/rs/zerolog"

	"github.com/DIMO-Network/edge-network/internal/api"

	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type FingerprintRunner interface {
	Fingerprint() error
	FingerprintSimple(powerStatus api.PowerStatusResponse) error
}

type fingerprintRunner struct {
	unitID        uuid.UUID
	vinLog        loggers.VINLogger
	dataSender    network.DataSender
	templateStore loggers.TemplateStore
	logger        zerolog.Logger
}

func NewFingerprintRunner(unitID uuid.UUID, vinLog loggers.VINLogger, dataSender network.DataSender, templateStore loggers.TemplateStore, logger zerolog.Logger) FingerprintRunner {
	return &fingerprintRunner{unitID: unitID, vinLog: vinLog, dataSender: dataSender, templateStore: templateStore, logger: logger}
}

const maxFailureAttempts = 5

// FingerprintSimple does no voltage checks, just scans for the VIN, saves query used, and sends cloud event signed.
func (ls *fingerprintRunner) FingerprintSimple(powerStatus api.PowerStatusResponse) error {
	// no voltage check, assumption is this has already been checked before here.
	config, err := ls.templateStore.ReadVINConfig()
	if err != nil {
		ls.logger.Info().Msgf("could not read settings, continuing: %s", err)
	}
	var vqn *string
	if config != nil {
		vqn = &config.VINQueryName
		ls.logger.Info().Msgf("found VIN query name: %s", *vqn)
	}
	// scan for VIN
	vinLogger := LoggerProperties{
		SignalName: "vin",
		Interval:   0,
		ScanFunc:   ls.vinLog.GetVIN,
	}
	// assumption here is that the vin query name, if set, will work on this car. If the device has been moved to a different car without pairing again, this won't work
	vinResp, err := vinLogger.ScanFunc(ls.unitID, vqn)
	if err != nil {
		if config == nil {
			config = &models.VINLoggerSettings{}
		}
		ls.logger.Err(err).Msgf("failed to scan for vin. fail count: %d", config.VINLoggerFailedAttempts)
		// update local settings to increment fail count
		config.VINLoggerVersion = loggers.VINLoggerVersion
		config.VINLoggerFailedAttempts++
		writeErr := ls.templateStore.WriteVINConfig(*config)
		if writeErr != nil {
			ls.logger.Err(writeErr).Send()
		}

		// todo: how can we minimize this logging to edge-logs
		_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to get VIN from vinLogger"), &powerStatus)
	}
	// save vin query name in settings if not set
	if config == nil || config.VINQueryName == "" {
		config = &models.VINLoggerSettings{VINQueryName: vinResp.QueryName, VIN: vinResp.VIN}
		err := ls.templateStore.WriteVINConfig(*config)
		if err != nil {
			ls.logger.Err(err).Send()
			_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to write vinLogger settings"), &powerStatus)
		}
	}

	data := models.FingerprintData{
		Vin:      vinResp.VIN,
		Protocol: vinResp.Protocol,
	}
	data.Device.RpiUptimeSecs = powerStatus.Rpi.Uptime.Seconds
	data.Device.BatteryVoltage = powerStatus.VoltageFound
	version, err := commands.GetSoftwareVersion(ls.unitID)
	if err == nil {
		data.SoftwareVersion = version
	}

	err = ls.dataSender.SendFingerprintData(data)
	if err != nil {
		ls.logger.Err(err).Send()
	}

	return nil
}

// Fingerprint checks if ok to start scanning the vehicle and then tries to get the VIN & Protocol via various methods.
// Runs only once when successful.  Checks for saved VIN query from previous run.
// Deprecated: Use FingerprintSimple instead. This one was trying to pack in more logic that is now handled by caller.
func (ls *fingerprintRunner) Fingerprint() error {
	// check if ok to start making obd calls etc
	ls.logger.Info().Msg("fingerprint starting, checking if can start scanning")
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
	config, err := ls.templateStore.ReadVINConfig()
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
			config = &models.VINLoggerSettings{}
		}
		ls.logger.Err(err).Msgf("failed to scan for vin. fail count: %d", config.VINLoggerFailedAttempts)
		// update local settings to increment fail count
		config.VINLoggerVersion = loggers.VINLoggerVersion
		config.VINLoggerFailedAttempts++
		writeErr := ls.templateStore.WriteVINConfig(*config)
		if writeErr != nil {
			ls.logger.Err(writeErr).Send()
		}
		_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to get VIN from vinLogger"), &status)

		return err
	} else if config == nil {
		// save vin query name in settings if not set
		config = &models.VINLoggerSettings{VINQueryName: vinResp.QueryName, VIN: vinResp.VIN}
		err := ls.templateStore.WriteVINConfig(*config)
		if err != nil {
			ls.logger.Err(err).Send()
			_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to write vinLogger settings"), &status)
		}
	}

	data := models.FingerprintData{
		Vin:      vinResp.VIN,
		Protocol: vinResp.Protocol,
	}
	data.Device.RpiUptimeSecs = status.Rpi.Uptime.Seconds
	data.Device.BatteryVoltage = status.Stn.Battery.Voltage

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
	const voltageMin = 13.2 // todo this should come from device config but have defaults
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

	ls.logger.Info().Msgf("loggers: Last Start Reason: %s - Voltage: %f", status.Spm.LastTrigger.Up, status.VoltageFound)
	if status.Spm.LastTrigger.Up == "volt_change" || status.Spm.LastTrigger.Up == "volt_level" || status.Spm.LastTrigger.Up == "stn" || status.Spm.LastTrigger.Up == "spm" {
		if status.VoltageFound >= voltageMin {
			// good to start scanning
			result = true
			return
		}
		// loop a few more times
		tries = 0
		for status.VoltageFound < voltageMin {
			if tries > maxTries {
				err = fmt.Errorf("loggers: did not reach a satisfactory voltage to start loggers: %f", status.Stn.Battery.Voltage)
				ls.logger.Err(err).Send()
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
		return false, status,
			fmt.Errorf("loggers: Spm.LastTrigger.Up value not expected so not starting logger: %s - Voltage: %f - required Voltage: %f",
				status.Spm.LastTrigger.Up, status.VoltageFound, voltageMin)
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
