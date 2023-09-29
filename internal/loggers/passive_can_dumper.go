package loggers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"go.einride.tech/can"
	"go.einride.tech/can/pkg/candevice"
	"go.einride.tech/can/pkg/socketcan"
	"os"
	"strconv"
	"time"
)

type ParsedCanFrame struct {
	FrameHex  string `json:"FrameHex"`
	FrameInt  int    `json:"FrameInt"`
	FrameData string `json:"Data"`
}

type MqttCandumpMessage struct {
	UnitId           string           `json:"UnitId,omitempty"`
	EthAddress       string           `json:"EthAddress,omitempty"`
	TimeStamp        string           `json:"TimeStamp,omitempty"`
	Page             int              `json:"Page,omitempty"`
	TotalPages       int              `json:"TotalPages,omitempty"`
	DeviceDefinition string           `json:"DeviceDefinition,omitempty"`
	RawData          []ParsedCanFrame `json:"RawData"`
}

type PassiveCanDumper struct {
	CapturedFrames       []can.Frame
	capturedFrameStrings []string
	DetailedCanFrames    []ParsedCanFrame
}

// WriteToMQTT This function writes the contents of PassiveCanDumper.DetailedCanFrames to an mqtt server,
//and also writes to local files. Can frames from memory will be automatically paginated into appropriate
//qty of messages/files according to chunkSize. Data is formatted as json, gzip compressed, then base64 compressed.
func (a *PassiveCanDumper) WriteToMQTT(UnitID uuid.UUID, EthAddr common.Address, chunkSize int, timeStamp string, writeToLocalFiles bool) error {
	unitId := UnitID.String()
	ethAddr := EthAddr.String()

	message := MqttCandumpMessage{
		UnitId:     unitId,
		EthAddress: ethAddr,
		TimeStamp:  timeStamp,
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

		if writeToLocalFiles {
			fileErr := os.WriteFile(timeStamp+"_page_"+strconv.Itoa(message.Page), payload, 666)
			if fileErr != nil {
				println(fileErr.Error())
				return fileErr
			}
		}

		ds := network.NewDataSender(UnitID, EthAddr, "protocol/canbus/dump")
		sendErr := ds.SendCanDumpData(network.CanDumpData{
			CommonData: network.CommonData{
				Timestamp: time.Now().UTC().UnixMilli(),
			},
			Payload: base64.StdEncoding.EncodeToString(buf.Bytes()),
		})

		if sendErr != nil {
			println("error sending")
			return sendErr
		}
		/*
			cmd := exec.Command("mosquitto_pub", "-h", hostname, "-t", topic, "-m", base64.StdEncoding.EncodeToString(buf.Bytes()))
			cmdout, _ := cmd.Output()
			println(string(cmdout))
		*/

		message.Page++
	}
	return nil
}

// WriteToFile WriteToFile() will write the can frames currently stored in memory to a single json file on local disk, without pagination
func (a *PassiveCanDumper) WriteToFile(filename string) error {
	if len(filename) < 1 {
		return errors.New("Invalid filename. Please use the following syntax:\n ./edge-network -candump <baudrate> <cycle_count> <file_out>")
	}
	outFile, _ := json.Marshal(a.DetailedCanFrames)
	//print(string(outFile))
	err := os.WriteFile(filename, outFile, 666)
	if err != nil {
		println("error writing to file: ", err)
		return err
	}
	return nil
}

// ReadCanBusTest This method is used for testing purposes, to simulate a can bus read
func (a *PassiveCanDumper) ReadCanBusTest(cycles int, bitrate int) {
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
	//return a.DetailedCanFrames
}

// ReadCanBus This function reads frames from the can bus and loads the data into memory. Data is populated to  *a.DetailedCanFrames[]
func (a *PassiveCanDumper) ReadCanBus(cycles int, bitrate int) error {
	d, _ := candevice.New("can0")
	_ = d.SetBitrate(uint32(bitrate))
	_ = d.SetUp()
	defer d.SetDown()

	conn, err := socketcan.DialContext(context.Background(), "can", "can0")
	if err != nil {
		return err
	}
	println("socketcan.DialContext()")

	recv := socketcan.NewReceiver(conn)
	println("socketcan.NewReceiver(conn)")
	var loopNumber = 0
	for recv.Receive() {
		loopNumber++
		if loopNumber > cycles {
			println("Cycles completed:", len(a.DetailedCanFrames))
			break
		}
		frame := recv.Frame()
		frameInt, frameHex, data := getValuesFromCanFrame(frame)

		a.DetailedCanFrames = append(a.DetailedCanFrames, ParsedCanFrame{
			FrameData: data, FrameHex: frameHex, FrameInt: frameInt,
		})
	}
	return nil
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
