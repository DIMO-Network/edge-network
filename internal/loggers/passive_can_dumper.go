package loggers

import (
	"context"
	"os"
	"strconv"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/candevice"
	"go.einride.tech/can/pkg/socketcan"
)

type PassiveCanDumper struct {
	CapturedFrames       []can.Frame
	capturedFrameStrings []string
}

func (a PassiveCanDumper) WriteToElastic(unitId string) {
	headerMap := make(map[string]string)
	for _, frame := range a.CapturedFrames {
		headerMap[strconv.Itoa(int(frame.ID))] = frame.String()
	}
	/*var payload string
	for header, _ := range headerMap {
		payload = " "
	}*/
}

func (a PassiveCanDumper) WriteToFile(filename string) {
	var outFile = ""
	for _, frame := range a.CapturedFrames {
		outFile += frame.String() + "\n"
		println("capturedFrame:", frame.String())
	}
	print(outFile)
	err := os.WriteFile(filename, []byte(outFile), 666)
	if err != nil {
		println("error writing to file: ", err)
	}
	/*
		var outFileStrings = ""
		for line := range a.capturedFrameStrings {
			outFileStrings += a.capturedFrameStrings[line] + "/n"
			println("capturedFrameStrings:", a.capturedFrameStrings[line])
		}
		print(outFileStrings)
		err = os.WriteFile("testcandumpstrings.txt", []byte(outFileStrings), 666)
		if err != nil {
			println("error writing to file: ", err)
		}*/
}

func (a PassiveCanDumper) ReadCanBus(cycles int, bitrate int) []can.Frame {
	d, _ := candevice.New("can0")
	println("can device created")
	_ = d.SetBitrate(uint32(bitrate))
	println("bitrate set to: ", bitrate)
	_ = d.SetUp()
	println("can device .SetUp()")
	defer d.SetDown() //nolint
	println("can device .SetDown() deferred")

	conn, _ := socketcan.DialContext(context.Background(), "can", "can0")
	println("socketcan.DialContext()")

	recv := socketcan.NewReceiver(conn)
	println("socketcan.NewReceiver(conn)")
	var loopNumber = 0
	a.CapturedFrames = make([]can.Frame, 0)
	for recv.Receive() {
		loopNumber++
		//println(loopNumber)
		frame := recv.Frame()
		println(frame.String())
		a.CapturedFrames = append(a.CapturedFrames, frame)
		a.capturedFrameStrings = append(a.capturedFrameStrings, frame.String())
		if loopNumber > cycles {
			println("Cycles completed:", loopNumber-1)
			break
		}
	}
	println("recv.Receive() loop exit")
	println("capturedFrameStrings:")
	return a.CapturedFrames
}
