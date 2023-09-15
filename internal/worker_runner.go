package internal

import (
	"log"
	"sync"

	"github.com/DIMO-Network/edge-network/internal/loggers"
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
}

func NewWorkerRunner(unitID uuid.UUID, loggerSettingsSvc loggers.LoggerSettingsService, pidLog loggers.PIDLogger, loggerSvc LoggerService) WorkerRunner {
	return &workerRunner{unitID: unitID, loggerSettingsSvc: loggerSettingsSvc, pidLog: pidLog, loggerSvc: loggerSvc}
}

func (wr *workerRunner) Run() {
	v, _ := wr.loggerSettingsSvc.ReadVINConfig()
	// only start PID loggers if have a VIN
	// todo: We'll need an option for cars where VIN comes from cloud b/c we couldn't get the VIN via OBD
	if len(v.VIN) > 0 {
		err := wr.loggerSvc.PIDLoggers(v.VIN)
		if err != nil {
			log.Printf("failed to pid loggers: %s \n", err.Error())
		}

		if err == nil {
			pidsConfig, _ := wr.loggerSettingsSvc.ReadPIDsConfig()
			tasks := make([]WorkerTask, len(pidsConfig.PIDs))

			for i, task := range pidsConfig.PIDs {
				tasks[i] = WorkerTask{
					Name:     task.Name,
					Interval: task.Interval,
					Once:     task.Interval == 0,
					Params: map[string]interface{}{
						"UnitID":   wr.unitID,
						"Header":   task.Header,
						"Mode":     task.Mode,
						"PID":      task.PID,
						"Formula":  task.Formula,
						"Protocol": task.Protocol,
					},
					Func: func(ctx WorkerTaskContext) {
						err := wr.pidLog.ExecutePID(ctx.Params["Header"].(string),
							ctx.Params["Mode"].(string),
							ctx.Params["PID"].(string),
							ctx.Params["Formula"].(string),
							ctx.Params["Protocol"].(string))
						if err != nil {
							log.Printf("failed execute pid loggers: %s \n", err.Error())
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

			var wg sync.WaitGroup

			for i, task := range tasks {
				wg.Add(1)
				go func(idx int, t WorkerTask) {
					defer wg.Done()
					t.Execute(idx)
				}(i, task)
			}

			wg.Wait()
		}

	}

}
