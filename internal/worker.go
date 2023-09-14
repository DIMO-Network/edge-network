package internal

import (
	log "github.com/sirupsen/logrus"
	"time"
)

type WorkerTask struct {
	Name          string
	Once          bool
	Executions    int
	MaxExecutions int
	Interval      int
	Params        map[string]interface{}
	Func          func(WorkerTaskContext)
}

type WorkerTaskContext struct {
	Name       string
	Executions int
	Params     map[string]interface{}
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

func (t *WorkerTask) Execute(idx int) {
	ctx := WorkerTaskContext{Name: t.Name, Params: t.Params}

	if !t.Once {
		for t.Executions < t.MaxExecutions || t.MaxExecutions == 0 {
			log.Printf("Start task %s: %d", t.Name, t.Executions)
			t.Func(ctx)
			log.Printf("End task %s: %d", t.Name, t.Executions)
			t.Executions++
			ctx.Executions++
			time.Sleep(time.Duration(t.Interval) * time.Second)
		}
	} else {
		log.Printf("Start task %s: %d", t.Name, t.Executions)
		t.Func(ctx)
		log.Printf("End task %s: %d", t.Name, t.Executions)
		t.Executions++
	}
}
