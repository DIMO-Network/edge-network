package loggers

import (
	"context"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/candevice"
	"go.einride.tech/can/pkg/socketcan"
)

type PassiveCanDumper struct {
	capturedFrames []can.Frame
}

func (a PassiveCanDumper) ReadCanBus(cycles int) {
	d, _ := candevice.New("can0")
	_ = d.SetBitrate(500000)
	_ = d.SetUp()
	defer d.SetDown() //nolint

	conn, _ := socketcan.DialContext(context.Background(), "can", "can0")

	recv := socketcan.NewReceiver(conn)
	var loopNumber = 0
	a.capturedFrames = make([]can.Frame, 0)
	for recv.Receive() {
		loopNumber++
		println(loopNumber)
		frame := recv.Frame()
		println(frame.String())
		a.capturedFrames = append(a.capturedFrames, frame)
		if loopNumber > cycles {
			println("Cycles completed:", loopNumber-1)
			break
		}
	}
}
