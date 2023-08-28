package loggers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/candevice"
	"go.einride.tech/can/pkg/socketcan"
)

type RawCanFrame struct {
	id        string
	frameData string
}

type MqttCandumpMessage struct {
	unitId     string
	timeStamp  string
	page       string
	totalPages string
	make       string
	model      string
	rawData    []RawCanFrame
}

type PassiveCanDumper struct {
	CapturedFrames       []can.Frame
	capturedFrameStrings []string
}

func (a PassiveCanDumper) MarshallJson() {
	mcdm := MqttCandumpMessage{
		unitId:     "unitId",
		page:       "1",
		totalPages: "5",
		make:       "makename",
		model:      "modelname"}
	var rcf []RawCanFrame
	rcf = append(rcf, RawCanFrame{
		id:        "123",
		frameData: "frdata"})
	rcf = append(rcf, RawCanFrame{
		id:        "456",
		frameData: "frdata2"})
	mcdm.rawData = rcf
	payload, err := json.Marshal(rcf)
	if err != nil {
		println("could not execute mosquitto_pub")
		println(err)
	} else {
		println(payload)
	}
	//var payload string
	/*
		headerMap := make(map[string]string)
		for _, frame := range a.CapturedFrames {
			headerMap[strconv.Itoa(int(frame.ID))] = frame.String()
		}
		for header, _ := range headerMap {
		rcf := &RawCanFrame{
			id: "123",
			frameData: "frdata"}
		payload, err := json.Marshal(rcf)
		if  err != nil {
			println("could not execute mosquitto_pub")
			println(err)
		}else{
			println(payload)
		}
	}*/
}

func (a PassiveCanDumper) WriteToElastic(unitId string) {
	headerMap := make(map[string]string)
	for _, frame := range a.CapturedFrames {
		headerMap[strconv.Itoa(int(frame.ID))] = frame.String()
	}
	var payload string
	for header, _ := range headerMap {

		payload = fmt.Sprintf("\"{\\\"%s\\\":\\\"%s\\\"}\"", "header_"+header, unitId)
		cmd := exec.Command("mosquitto_pub", "-t", "reactor", "-m", payload)

		//cmd := exec.Command("mosquitto_pub", "-t", "reactor", "-m", "\"{\\\"canbus_testparam\\\":\\\"123TEST\\\"}\"")

		//cmd := exec.Command("mosquitto_pub", "-t", "reactor", "-m", "\"{\\\"canbus_testparam\\\":\\\"123TEST\\\"}\"")
		//"{\"canbus_testparam\":\"123TEST\"}"
		_, err := cmd.Output()
		if err != nil {
			println("could not execute mosquitto_pub")
			println(err)
		}
	}
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
