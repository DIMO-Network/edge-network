package internal

import (
	"time"

	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/rs/zerolog"
)

type WorkerTask struct {
	Name          string
	Once          bool
	Executions    int
	MaxExecutions int
	Interval      int
	Params        models.PIDRequest
	Func          func(WorkerTaskContext)
}

type WorkerTaskContext struct {
	Name       string
	Executions int
	Params     models.PIDRequest
}

func (t *WorkerTask) Register() {
	ctx := WorkerTaskContext{Name: t.Name, Params: t.Params}

	if !t.Once {
		for t.Executions < t.MaxExecutions {
			t.Func(ctx)
			t.Executions++
			ctx.Executions++
			time.Sleep(time.Duration(t.Interval) * time.Second)
		}
	} else {
		t.Func(ctx)
		t.Executions++
	}
}

func (t *WorkerTask) Execute(logger zerolog.Logger) {
	ctx := WorkerTaskContext{Name: t.Name, Params: t.Params}
	// todo could have a check here to make sure isOkToScan (eg. using same code as in fingerprint stuff)
	if !t.Once {
		for t.Executions < t.MaxExecutions || t.MaxExecutions == 0 {
			logger.Debug().Msgf("Start task %s: %d", t.Name, t.Executions)
			t.Func(ctx)
			logger.Debug().Msgf("End task %s: %d", t.Name, t.Executions)
			t.Executions++
			ctx.Executions++
			time.Sleep(time.Duration(t.Interval) * time.Second)
		}
	} else {
		logger.Debug().Msgf("Start task %s: %d", t.Name, t.Executions)
		t.Func(ctx)
		logger.Debug().Msgf("End task %s: %d", t.Name, t.Executions)
		t.Executions++
	}
}
