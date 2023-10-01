package internal

import (
	"github.com/rs/zerolog"
	"sync"
	"time"

	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/DIMO-Network/edge-network/internal/queue"
	"github.com/google/uuid"
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
}

func NewWorkerRunner(unitID uuid.UUID, loggerSettingsSvc loggers.LoggerSettingsService, pidLog loggers.PIDLogger, loggerSvc LoggerService, queueSvc queue.StorageQueue, dataSender network.DataSender, logger zerolog.Logger) WorkerRunner {
	return &workerRunner{unitID: unitID, loggerSettingsSvc: loggerSettingsSvc, pidLog: pidLog, loggerSvc: loggerSvc, queueSvc: queueSvc, dataSender: dataSender, logger: logger}
}

func (wr *workerRunner) Run() {
	var tasks []WorkerTask

	pidTasks := wr.registerPIDsTasks()
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

	tasks = append(tasks, WorkerTask{
		Name:     "Sender Task",
		Interval: 60,
		Func: func(ctx WorkerTaskContext) {
			for {
				messages, err := wr.queueSvc.Dequeue()
				if err != nil {
					wr.logger.Info().Msgf("failed to queue pids: %s \n", err.Error())
					break
				}
				if len(messages) == 0 {
					break
				}
				signals := make([]network.SignalData, len(messages))
				for i, message := range messages {
					signals[i] = network.SignalData{Time: message.Time, Name: message.Name, Value: message.Content}
				}
				err = wr.dataSender.SendDeviceStatusData(network.DeviceStatusData{
					Time:    time.Now(),
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

func (wr *workerRunner) registerPIDsTasks() []WorkerTask {

	v, _ := wr.loggerSettingsSvc.ReadVINConfig()
	// only start PID loggers if have a VIN
	// todo: We'll need an option for cars where VIN comes from cloud b/c we couldn't get the VIN via OBD
	if len(v.VIN) > 0 {
		err := wr.loggerSvc.PIDLoggers(v.VIN)
		if err != nil {
			wr.logger.Info().Msgf("failed to pid loggers: %s \n", err.Error())
		}

		if err == nil {
			pidsConfig, _ := wr.loggerSettingsSvc.ReadPIDsConfig()
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
	}

	return []WorkerTask{}
}
