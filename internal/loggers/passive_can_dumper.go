package loggers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go.einride.tech/can"
	"go.einride.tech/can/pkg/candevice"
	"go.einride.tech/can/pkg/socketcan"
	"io"
	"os"
	"os/exec"
	"strconv"
)

type ParsedCanFrame struct {
	FrameHex  string `json:"FrameHex"`
	FrameInt  int    `json:"FrameInt"`
	FrameData string `json:"Data"`
}

type MqttCandumpMessage struct {
	UnitId           string           `json:"UnitId,omitempty"`
	TimeStamp        string           `json:"TimeStamp,omitempty"`
	Page             int              `json:"Page,omitempty"`
	TotalPages       int              `json:"TotalPages,omitempty"`
	DeviceDefinition string           `json:"DeviceDefinition,omitempty"`
	RawData          []ParsedCanFrame `json:"RawData"`
}

/*
type MqttCandumpMessageWithCanFrames struct {
	UnitId     string      `json:"UnitId,omitempty"`
	TimeStamp  string      `json:"TimeStamp,omitempty"`
	Page       string      `json:"Page,omitempty"`
	TotalPages string      `json:"TotalPages,omitempty"`
	Make       string      `json:"Make,omitempty"`
	Model      string      `json:"Model,omitempty"`
	RawData    []can.Frame `json:"RawData"`
}*/

type PassiveCanDumper struct {
	CapturedFrames       []can.Frame
	capturedFrameStrings []string
	DetailedCanFrames    []ParsedCanFrame
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

func (a PassiveCanDumper) TestMQTT() {
	var payload []byte
	var err error
	mcdm := MqttCandumpMessage{
		UnitId:     "unitId",
		Page:       1,
		TotalPages: 5,
	}
	var rcf []ParsedCanFrame

	rcf = append(rcf, *new(ParsedCanFrame))

	mcdm.RawData = rcf
	payload, err = json.Marshal(mcdm)
	if err != nil {
		println("could not execute mosquitto_pub")
		println(err)
	} else {
		//println(string(payload))
	}
	cmd := exec.Command("mosquitto_pub", "-h", "test.mosquitto.org", "-t", "testtopic", "-m", string(payload))
	println(cmd.Output())
	var bufstr bytes.Buffer
	notgz := io.Writer(&bufstr)
	_, _ = notgz.Write(payload)
	//_ = notgz.Close()
	cmd = exec.Command("mosquitto_pub", "-h", "test.mosquitto.org", "-t", "testtopic", "-m", base64.StdEncoding.EncodeToString(bufstr.Bytes()))
	println(cmd.Output())
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(payload)
	_ = gz.Close()
	cmd = exec.Command("mosquitto_pub", "-h", "test.mosquitto.org", "-t", "testtopic", "-m", base64.StdEncoding.EncodeToString(buf.Bytes()))
	println(cmd.Output())
}

func (a PassiveCanDumper) WriteToMQTT(unitId string, hostname string, topic string, chunkSize int, timeStamp string) {
	message := MqttCandumpMessage{
		UnitId:    unitId,
		TimeStamp: timeStamp,
	}

	totalPages := len(a.DetailedCanFrames) / chunkSize
	if (len(a.DetailedCanFrames) % chunkSize) > 0 {
		totalPages++
	}
	message.TotalPages = totalPages
	message.Page = 1

	var payload []byte
	var _ error
	for i := 0; i < len(a.DetailedCanFrames); i += chunkSize {
		if len(a.DetailedCanFrames) > i+chunkSize {
			message.RawData = a.DetailedCanFrames[i : i+chunkSize]
		} else {
			message.RawData = a.DetailedCanFrames[i:len(a.DetailedCanFrames)]
		}
		payload, _ = json.Marshal(message)
		println(string(payload))
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, _ = gz.Write(payload)
		_ = gz.Close()
		cmd := exec.Command("mosquitto_pub", "-h", hostname, "-t", topic, "-m", base64.StdEncoding.EncodeToString(buf.Bytes()))
		cmdout, _ := cmd.Output()
		println(string(cmdout))

		fileErr := os.WriteFile(timeStamp+"_page_"+strconv.Itoa(message.Page), payload, 776)
		if fileErr != nil {
			println(fileErr.Error())
			os.Exit(0)
		}
		message.Page++
	}
}

func (a PassiveCanDumper) WriteToFile(filename string) {
	var outFile = ""
	for _, frame := range a.CapturedFrames {
		outFile += frame.String() + "\n"
		println("capturedFrame:", frame.String())

		frame_json, _ := frame.MarshalJSON() //these lines are only to test
		println(string(frame_json))          // test only

	}
	print(outFile)
	err := os.WriteFile(filename, []byte(outFile), 666)
	if err != nil {
		println("error writing to file: ", err)
	}
}

func (a PassiveCanDumper) ReadCanBusTest(cycles int, bitrate int) []ParsedCanFrame {
	//d, _ := candevice.New("can0")
	println("can device created")
	//_ = d.SetBitrate(uint32(bitrate))
	println("bitrate set to: ", bitrate)
	//_ = d.SetUp()
	println("can device .SetUp()")
	//defer d.SetDown() //nolint
	println("can device .SetDown() deferred")

	//conn, _ := socketcan.DialContext(context.Background(), "can", "can0")
	println("socketcan.DialContext()")

	//recv := socketcan.NewReceiver(conn)
	println("socketcan.NewReceiver(conn)")
	var loopNumber = 0
	a.DetailedCanFrames = *new([]ParsedCanFrame)
	//a.CapturedFrames = make([]can.Frame, 0)
	for true {
		//for recv.Receive() {
		loopNumber++
		//frame := recv.Frame()
		frame := *new(can.Frame)

		frameInt, frameHex, data := getValuesFromCanFrame(frame)

		if loopNumber > cycles {
			println("Cycles completed:", loopNumber-1)
			break
		}
		a.DetailedCanFrames = append(a.DetailedCanFrames, ParsedCanFrame{
			FrameData: data, FrameHex: frameHex, FrameInt: frameInt,
		})
	}
	return a.DetailedCanFrames
}

func (a PassiveCanDumper) ReadCanBus(cycles int, bitrate int) []ParsedCanFrame {
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
	a.DetailedCanFrames = *new([]ParsedCanFrame)
	//a.CapturedFrames = *new([]can.Frame)
	//a.CapturedFrames = make([]can.Frame, 0)
	for recv.Receive() {
		loopNumber++
		if loopNumber > cycles {
			println("Cycles completed:", len(a.DetailedCanFrames))
			break
		}
		frame := recv.Frame()

		//println(frame.String())

		//a.CapturedFrames = append(a.CapturedFrames, frame)

		frameInt, frameHex, data := getValuesFromCanFrame(frame)

		a.DetailedCanFrames = append(a.DetailedCanFrames, ParsedCanFrame{
			FrameData: data, FrameHex: frameHex, FrameInt: frameInt,
		})
	}

	println("recv.Receive() loop exit")
	println("capturedFrameStrings:")
	return a.DetailedCanFrames
}

func getValuesFromCanFrame(frame can.Frame) (frameInt int, frameHex string, data string) {
	fullStr := frame.String()
	//fullStr := "215#2710271027102710"
	i := 0
	for i < len(fullStr) {
		if fullStr[i] == '#' {
			break
		}
		i++
	}
	frameHex = fullStr[0:i]
	data = fullStr[i+1:]
	//IntVal, _ := strconv.ParseInt(frameHex, 16, 32)
	IntVal := frame.ID
	return int(IntVal), frameHex, data
}
