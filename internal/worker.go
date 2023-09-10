package internal

import "time"

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

func (t *WorkerTask) Execute(idx int) {
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
