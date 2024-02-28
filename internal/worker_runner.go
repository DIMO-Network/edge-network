package internal

import (
	"fmt"
	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/DIMO-Network/edge-network/internal/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"sync"
	"time"
)

type WorkerRunner interface {
	Run()
}

type workerRunner struct {
	unitID            uuid.UUID
	loggerSettingsSvc loggers.TemplateStore
	pidLog            loggers.PIDLogger
	queueSvc          queue.StorageQueue
	dataSender        network.DataSender
	logger            zerolog.Logger
	ethAddr           *common.Address
	fingerprintRunner FingerprintRunner
	pids              *models.TemplatePIDs
	deviceSettings    *models.TemplateDeviceSettings
}

func NewWorkerRunner(unitID uuid.UUID, addr *common.Address, loggerSettingsSvc loggers.TemplateStore, pidLog loggers.PIDLogger,
	queueSvc queue.StorageQueue, dataSender network.DataSender, logger zerolog.Logger, fpRunner FingerprintRunner, pids *models.TemplatePIDs, settings *models.TemplateDeviceSettings) WorkerRunner {
	return &workerRunner{unitID: unitID, ethAddr: addr, loggerSettingsSvc: loggerSettingsSvc, pidLog: pidLog,
		queueSvc: queueSvc, dataSender: dataSender, logger: logger, fingerprintRunner: fpRunner, pids: pids, deviceSettings: settings}
}

// Run sends a signed status payload every X seconds, that may or may not contain OBD signals.
// It also has a continuous loop that checks voltage compared to template settings to make sure ok to query OBD.
// It will query the VIN once on startup and send a fingerprint payload (only once per Run).
// If ok to query OBD, queries each signal per it's designated interval.
func (wr *workerRunner) Run() {
	// todo v1: if no template settings obtained, we just want to send the status payload without obd stuff.

	vin, err := wr.loggerSettingsSvc.ReadVINConfig()
	if err != nil {
		wr.logger.Err(err).Msg("unable to get vin for worker runner from vehicle")
	}
	wr.logger.Info().Msgf("starting worker runner with vin: %s", vin.VIN)

	wr.logger.Info().Msgf("starting worker runner with logger settings: %+v", *wr.deviceSettings)

	modem, err := commands.GetModemType(wr.unitID)
	if err != nil {
		modem = "ec2x"
		wr.logger.Err(err).Msg("unable to get modem type, defaulting to ec2x")
	}
	wr.logger.Info().Msgf("found modem: %s", modem)

	// we will need two clocks, one for non-obd (every 20s) and one for obd (continuous, based on each signal interval)
	// which clock checks batteryvoltage? we want to send it with every status payload
	// register tasks that can be iterated over
	fingerprintDone := false
	for {
		// naive implementation, just get messages sending - important to map out all actions. Later figure out right task engine
		queryOBD, powerStatus := wr.isOkToQueryOBD()
		// maybe start a timer here to know how long this cycle takes?
		// start a cloudevent
		signals := make([]network.SignalData, 1)
		signals[0] = network.SignalData{
			Timestamp: time.Now().UTC().UnixMilli(),
			Name:      "batteryVoltage",
			Value:     fmt.Sprintf("%f", powerStatus.VoltageFound),
		}
		// run through non obd ones: altitude, latitude, longitude, wifi connection, nsat (number gps satellites), cell signal info
		// should probably have function that just returns list of signals to refactor this
		wifiStatus, err := commands.GetWifiStatus(wr.unitID)
		if err != nil {
			wr.logger.Err(err).Msg("failed to get signal strength")
		} else {
			signals = append(signals, network.SignalData{
				Timestamp: time.Now().UTC().UnixMilli(),
				Name:      "wifi",
				Value:     wifiStatus,
			})
		}
		location, err := commands.GetGPSLocation(wr.unitID, modem)
		if err != nil {
			wr.logger.Err(err).Msg("failed to get gps location")
		} else {
			ts := time.Now().UTC().UnixMilli()
			signals = append(signals, network.SignalData{
				Timestamp: ts,
				Name:      "hdop",
				Value:     location.Hdop,
			})
			signals = append(signals, network.SignalData{
				Timestamp: ts,
				Name:      "nsat",
				Value:     location.NsatGPS,
			})
			signals = append(signals, network.SignalData{
				Timestamp: ts,
				Name:      "latitude",
				Value:     location.Lat,
			})
			signals = append(signals, network.SignalData{
				Timestamp: ts,
				Name:      "longitude",
				Value:     location.Lon,
			})
		}
		cellInfo, err := commands.GetQMICellInfo(wr.unitID)
		if err != nil {
			wr.logger.Err(err).Msg("failed to get qmi cell info")
		} else {
			// todo massage format to match what we put in elastic
			signals = append(signals, network.SignalData{
				Timestamp: time.Now().UTC().UnixMilli(),
				Name:      "cell",
				Value:     cellInfo,
			})
		}

		if queryOBD {
			// do fingerprint but only once
			if !fingerprintDone {
				err := wr.fingerprintRunner.FingerprintSimple(powerStatus)
				if err != nil {
					wr.logger.Err(err).Msg("failed to do vehicle fingerprint")
				} else {
					fingerprintDone = true
				}
			}
			// run through all the obd pids (ignoring the interval? for now), maybe have a function that executes them from previously registered
		}

		// send the cloud event
		// at the very end, wait for next loop
		s := network.DeviceStatusData{
			CommonData: network.CommonData{
				RpiUptimeSecs:  powerStatus.Rpi.Uptime.Seconds,
				BatteryVoltage: powerStatus.VoltageFound,
				Timestamp:      time.Now().UTC().UnixMilli(),
			},
			Signals: signals,
		}
		err = wr.dataSender.SendDeviceStatusData(s)
		if err != nil {
			wr.logger.Err(err).Msg("failed to send device status in loop")
		}
		time.Sleep(20 * time.Second)
	}

	var tasks []WorkerTask

	pidTasks := wr.registerPIDsTasks(*wr.pids)
	for _, task := range pidTasks {
		tasks = append(tasks, task)
	}

	senderTasks := wr.registerSenderTasks()
	for _, task := range senderTasks {
		tasks = append(tasks, task)
	}

	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t WorkerTask) {
			defer wg.Done()
			t.Execute(idx, wr.logger)
		}(i, task)
	}

	wg.Wait()
	wr.logger.Debug().Msg("worker Run completed")
}

func (wr *workerRunner) registerSenderTasks() []WorkerTask {
	var tasks []WorkerTask
	// build up task that sends mqtt payload every 60 seconds
	tasks = append(tasks, WorkerTask{
		Name:     "Sender Task",
		Interval: 60,
		Func: func(ctx WorkerTaskContext) {
			for {
				// are there many data points in one message? or is each message one signal data point
				messages, err := wr.queueSvc.Dequeue()
				if err != nil {
					wr.logger.Err(err).Msg("failed to Dequeue vehicle data signals")
					break
				}
				if len(messages) == 0 {
					break
				}
				// todo: does this result in the right cloudevent formatted message? or do we need to use sjson.Set
				signals := make([]network.SignalData, len(messages))
				for i, message := range messages {
					signals[i] = network.SignalData{Timestamp: message.Time.UnixMilli(), Name: message.Name, Value: message.Content}
				}
				err = wr.dataSender.SendDeviceStatusData(network.DeviceStatusData{
					Signals: signals,
				})
				if err != nil {
					wr.logger.Err(err).Msg("Unable to send device status data")
				}
			}
		},
	})

	return tasks
}

func (wr *workerRunner) registerPIDsTasks(pidsConfig models.TemplatePIDs) []WorkerTask {

	if len(pidsConfig.Requests) > 0 {
		tasks := make([]WorkerTask, len(pidsConfig.Requests))

		for i, task := range pidsConfig.Requests {
			tasks[i] = WorkerTask{
				Name:     task.Name,
				Interval: task.IntervalSeconds,
				Once:     task.IntervalSeconds == 0,
				Params:   task,
				Func: func(wCtx WorkerTaskContext) {
					err := wr.pidLog.ExecutePID(wCtx.Params.Header,
						wCtx.Params.Mode,
						wCtx.Params.Pid,
						wCtx.Params.Formula,
						wCtx.Params.Protocol,
						wCtx.Params.Name)
					if err != nil {
						wr.logger.Err(err).Msg("failed execute pid loggers:" + wCtx.Params.Name)
					}
				},
			}
		}

		// Execute Message
		tasks[len(tasks)+1] = WorkerTask{
			Name:     "Notify Message",
			Interval: 30,
			Func: func(ctx WorkerTaskContext) {

			},
		}

		return tasks
	}

	return []WorkerTask{}
}

// isOkToQueryOBD checks once to see if voltage rules pass to issue PID requests
func (wr *workerRunner) isOkToQueryOBD() (bool, api.PowerStatusResponse) {
	status, err := commands.GetPowerStatus(wr.unitID)
	if err != nil {
		wr.logger.Err(err).Msg("failed to get powerStatus for worker runner check")
		return false, status
	}
	if status.VoltageFound >= wr.deviceSettings.MinVoltageOBDLoggers {
		return true, status
	}
	return false, status
}
