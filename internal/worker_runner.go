package internal

import (
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/DIMO-Network/edge-network/internal/queue"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"sync"
)

type WorkerRunner interface {
	Run()
}

type workerRunner struct {
	unitID            uuid.UUID
	loggerSettingsSvc loggers.LoggerSettingsService
	pidLog            loggers.PIDLogger
	loggerSvc         LoggerService
	queueSvc          queue.StorageQueue
	dataSender        network.DataSender
	logger            zerolog.Logger
	vehicleTemplates  VehicleTemplates
}

func NewWorkerRunner(unitID uuid.UUID, loggerSettingsSvc loggers.LoggerSettingsService, pidLog loggers.PIDLogger, loggerSvc LoggerService, queueSvc queue.StorageQueue, dataSender network.DataSender, logger zerolog.Logger, templates VehicleTemplates) WorkerRunner {
	return &workerRunner{unitID: unitID, loggerSettingsSvc: loggerSettingsSvc, pidLog: pidLog, loggerSvc: loggerSvc, queueSvc: queueSvc, dataSender: dataSender, logger: logger, vehicleTemplates: templates}
}

func (wr *workerRunner) Run() {

	vin, err := wr.loggerSettingsSvc.ReadVINConfig()
	if err != nil {
		wr.logger.Err(err).Msg("unable to start worker runner b/c can't get VIN")
		return
	}
	wr.logger.Info().Msgf("starting worker runner with vin: %s", vin.VIN)

	loggerSettings, err := wr.vehicleTemplates.GetTemplateSettings(vin.VIN)
	if err != nil {
		// this means we really cannot start
		wr.logger.Err(err).Msg("unable to start worker runner b/c can't get vehicle template")
		return
	}
	wr.logger.Info().Msgf("starting worker runner with logger settings: %+v", *loggerSettings)

	var tasks []WorkerTask

	pidTasks := wr.registerPIDsTasks(*loggerSettings)
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
				messages, err := wr.queueSvc.Dequeue()
				if err != nil {
					wr.logger.Err(err).Msg("failed to Dequeue vehicle data signals")
					break
				}
				if len(messages) == 0 {
					break
				}
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

func (wr *workerRunner) registerPIDsTasks(pidsConfig loggers.PIDLoggerSettings) []WorkerTask {

	if len(pidsConfig.PIDs) > 0 {
		tasks := make([]WorkerTask, len(pidsConfig.PIDs))

		for i, task := range pidsConfig.PIDs {
			tasks[i] = WorkerTask{
				Name:     task.Name,
				Interval: task.Interval,
				Once:     task.Interval == 0,
				Params:   task,
				Func: func(wCtx WorkerTaskContext) {
					err := wr.pidLog.ExecutePID(wCtx.Params.Header,
						wCtx.Params.Mode,
						wCtx.Params.PID,
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
