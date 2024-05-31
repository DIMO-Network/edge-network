package internal

import (
	"context"
	"fmt"
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
	FingerprintSimple(powerStatus api.PowerStatusResponse) error
	CurrentFailureCount() int
	IncrementFailuresReached() int
}

type fingerprintRunner struct {
	unitID        uuid.UUID
	vinLog        loggers.VINLogger
	dataSender    network.DataSender
	templateStore loggers.TemplateStore
	logger        zerolog.Logger
	// state tracking
	failureCount int
	// allTimeFailureCount is loaded from disk from last boot - used to determine edge-logging need. Increments by 1 per boot cycle even if retried many times in one boot
	allTimeFailureCount int
	// pastVINQueryName is loaded from disk from last boot - used to speedup VIN request if we already know the method that worked last
	pastVINQueryName *string
}

func NewFingerprintRunner(unitID uuid.UUID, vinLog loggers.VINLogger, dataSender network.DataSender, templateStore loggers.TemplateStore, logger zerolog.Logger) FingerprintRunner {
	fpr := &fingerprintRunner{unitID: unitID, vinLog: vinLog, dataSender: dataSender, templateStore: templateStore, logger: logger}
	fpr.failureCount = 0
	fpr.allTimeFailureCount = 0

	pastVINInfo, err := fpr.templateStore.ReadVINConfig()
	if err != nil {
		fpr.logger.Info().Msgf("could not read settings, continuing: %s", err)
	} else {
		fpr.allTimeFailureCount = pastVINInfo.VINLoggerFailedAttempts

		if pastVINInfo.VINQueryName != "" {
			fpr.pastVINQueryName = &pastVINInfo.VINQueryName
			fpr.logger.Debug().Msgf("found previous VIN query name: %s", pastVINInfo.VINQueryName)
		}
	}
	return fpr
}

func (ls *fingerprintRunner) CurrentFailureCount() int {
	return ls.failureCount
}

// IncrementFailuresReached update the templateStore VIN info config with in incremented failure count and writes it to disk
func (ls *fingerprintRunner) IncrementFailuresReached() int {
	config := &models.VINLoggerSettings{
		VINLoggerVersion:        loggers.VINLoggerVersion,
		VINLoggerFailedAttempts: ls.allTimeFailureCount + 1,
	}
	writeErr := ls.templateStore.WriteVINConfig(*config)
	if writeErr != nil {
		ls.logger.Err(writeErr).Send()
	}
	return config.VINLoggerFailedAttempts
}

// FingerprintSimple does no voltage checks, just scans for the VIN, saves query used, and sends cloud event signed.
// if failure getting vin, logs to console and updates the failure count on disk - wonder if this is necessary
func (ls *fingerprintRunner) FingerprintSimple(powerStatus api.PowerStatusResponse) error {
	// no voltage check, assumption is this has already been checked before here.
	// scan for VIN
	vinLogger := LoggerProperties{
		SignalName: "vin",
		Interval:   0,
		ScanFunc:   ls.vinLog.GetVIN,
	}
	// assumption here is that the vin query name, if set, will work on this car. If the device has been moved to a different car without pairing again, this won't work
	vinResp, err := vinLogger.ScanFunc(ls.unitID, ls.pastVINQueryName)
	if err != nil {
		ls.failureCount++
		// just return the error here and let the caller save to disk + log to edge etc
		return errors.Wrap(err, fmt.Sprintf("failed to scan for vin. fail count since boot: %d", ls.failureCount))
	}
	// save vin query name in settings & report to edge logs if not set - normally this should only happen once with a given car.
	if ls.pastVINQueryName == nil {
		config := &models.VINLoggerSettings{VINQueryName: vinResp.QueryName, VIN: vinResp.VIN}
		err := ls.templateStore.WriteVINConfig(*config)
		if err != nil {
			ls.logger.Err(err).Send()
			_ = ls.dataSender.SendErrorPayload(errors.Wrap(err, "failed to write vinLogger settings"), &powerStatus)
		}
		ls.logger.Info().Ctx(context.WithValue(context.Background(), LogToMqtt, "true")).Msgf("succesfully obtained VIN via fingerprint")
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

type LoggerProperties struct {
	// SignalName name of signal to be published over mqtt
	SignalName string
	// Interval is how often to run. 0 means only on start
	Interval int32
	// Function to call to get the data from the vehicle
	ScanFunc func(uuid2 uuid.UUID, qn *string) (*loggers.VINResponse, error)
}
