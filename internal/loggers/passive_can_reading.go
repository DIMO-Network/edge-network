package loggers

import (
	"context"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/candevice"
	"go.einride.tech/can/pkg/socketcan"
)

type passiveVinReader struct {
	CitroenVinTypeAsegAfound, CitroenVinTypeAsegBfound, CitroenVinTypeAsegCfound                bool
	CitroenVinTypeBsegAfound, CitroenVinTypeBsegBfound, CitroenVinTypeBsegCfound                bool
	stopRunning                                                                                 bool
	CitroenVinTypeAacceptedFrameA, CitroenVinTypeAacceptedFrameB, CitroenVinTypeAacceptedFrameC can.Frame
	CitroenVinTypeBacceptedFrameA, CitroenVinTypeBacceptedFrameB, CitroenVinTypeBacceptedFrameC can.Frame
	completeVIN                                                                                 string
	VinType                                                                                     string
	detectedProtocol                                                                            int
	CitroenVinTypeAsegA, CitroenVinTypeAsegB, CitroenVinTypeAsegC                               string
	CitroenVinTypeBsegA, CitroenVinTypeBsegB, CitroenVinTypeBsegC                               string
}

func newPassiveVinReader() *passiveVinReader {
	x := new(passiveVinReader)
	x.CitroenVinTypeAsegAfound, x.CitroenVinTypeAsegBfound, x.CitroenVinTypeAsegCfound = false, false, false
	x.CitroenVinTypeAsegA = ""
	x.CitroenVinTypeAsegB = ""
	x.CitroenVinTypeAsegC = ""
	x.CitroenVinTypeBsegA, x.CitroenVinTypeBsegB, x.CitroenVinTypeBsegC = "", "", ""
	x.CitroenVinTypeBsegAfound, x.CitroenVinTypeBsegBfound, x.CitroenVinTypeBsegCfound = false, false, false
	x.detectedProtocol = 0
	x.stopRunning = false
	return &passiveVinReader{}
}

func (a *passiveVinReader) ReadCitroenVIN(cycles int) (string, int, string) {
	d, _ := candevice.New("can0")
	_ = d.SetBitrate(500000)
	_ = d.SetUp()
	defer d.SetDown() //nolint

	conn, _ := socketcan.DialContext(context.Background(), "can", "can0")

	recv := socketcan.NewReceiver(conn)
	var loopNumber = 0
	for recv.Receive() {
		loopNumber++
		frame := recv.Frame()
		//println(frame.String())
		if frame.ID == 0x4d2 && !a.CitroenVinTypeAsegAfound {
			a.CitroenVinTypeAacceptedFrameA = frame
			println("ACCEPTED: " + frame.String())
			a.CitroenVinTypeAsegAfound = true
		} else if frame.ID == 0x492 && !a.CitroenVinTypeAsegBfound {
			a.CitroenVinTypeAacceptedFrameB = frame
			println("ACCEPTED: " + frame.String())
			a.CitroenVinTypeAsegBfound = true
		} else if frame.ID == 0x4b2 && !a.CitroenVinTypeAsegCfound {
			a.CitroenVinTypeAacceptedFrameC = frame
			println("ACCEPTED: " + frame.String())
			a.CitroenVinTypeAsegCfound = true
		} else if frame.ID == 0x0814C201 && frame.Data[0] == 0x00 && !a.CitroenVinTypeBsegAfound {
			a.CitroenVinTypeBacceptedFrameA = frame
			println("ACCEPTED: " + frame.String())
			a.CitroenVinTypeBsegAfound = true
		} else if frame.ID == 0x0814C201 && frame.Data[0] == 0x01 && !a.CitroenVinTypeBsegBfound {
			a.CitroenVinTypeBacceptedFrameB = frame
			println("ACCEPTED: " + frame.String())
			a.CitroenVinTypeBsegBfound = true
		} else if frame.ID == 0x0814C201 && frame.Data[0] == 0x02 && !a.CitroenVinTypeBsegCfound {
			a.CitroenVinTypeBacceptedFrameC = frame
			println("ACCEPTED: " + frame.String())
			a.CitroenVinTypeBsegCfound = true
		}
		if a.CitroenVinTypeAsegAfound && a.CitroenVinTypeAsegBfound && a.CitroenVinTypeAsegCfound {
			for i := 0; i < 3; i++ {
				a.CitroenVinTypeAsegA += string(a.CitroenVinTypeAacceptedFrameA.Data[i])
			}
			for i := 0; i < 6; i++ {
				a.CitroenVinTypeAsegB += string(a.CitroenVinTypeAacceptedFrameB.Data[i])
			}
			for i := 0; i < 8; i++ {
				a.CitroenVinTypeAsegC += string(a.CitroenVinTypeAacceptedFrameC.Data[i])
			}
			a.completeVIN = a.CitroenVinTypeAsegA + a.CitroenVinTypeAsegB + a.CitroenVinTypeAsegC
			a.detectedProtocol = 6
			a.VinType = "CitroenVinTypeA"
			a.stopRunning = true
		} else if a.CitroenVinTypeBsegAfound && a.CitroenVinTypeBsegBfound && a.CitroenVinTypeBsegCfound {
			for i := 1; i < 8; i++ {
				a.CitroenVinTypeBsegA += string(a.CitroenVinTypeBacceptedFrameA.Data[i])
				println(string(a.CitroenVinTypeBacceptedFrameA.Data[i]))
			}
			for i := 1; i < 8; i++ {
				a.CitroenVinTypeBsegB += string(a.CitroenVinTypeBacceptedFrameB.Data[i])
			}
			for i := 1; i < 4; i++ {
				a.CitroenVinTypeBsegC += string(a.CitroenVinTypeBacceptedFrameC.Data[i])
			}
			a.completeVIN = a.CitroenVinTypeBsegA + a.CitroenVinTypeBsegB + a.CitroenVinTypeBsegC
			a.detectedProtocol = 7
			a.VinType = "CitroenVinTypeB"
			a.stopRunning = true
		}
		if loopNumber > cycles || a.stopRunning {
			break
		}
	}
	//println("message count:")
	//println(loopNumber)
	if a.CitroenVinTypeAsegAfound && a.CitroenVinTypeAsegBfound && a.CitroenVinTypeAsegCfound {
		return a.completeVIN, a.detectedProtocol, a.VinType
	} else if a.CitroenVinTypeBsegAfound && a.CitroenVinTypeBsegBfound && a.CitroenVinTypeBsegCfound {
		return a.completeVIN, a.detectedProtocol, a.VinType
	}
	return "", 0, ""
}

/*
VIN SAMPLES:
			CitroenVinTypeA:
					Models: 2017 Citroën Berlingo Multispace 2017 (ex. VF77FBHY6HJ734213)
								//ex. "autoPiUnitId": "3ba9494d-22bc-249e-616c-dc4e1ebc45d4",
							Citroen C3 2021 -
								//ex. 7768ab5d-ce1b-f0de-2178-687c836daae4
					Protocol: 6
					Sample VIN data:
						CitroenVinTypeAsegA:   4d2#564637
						CitroenVinTypeAsegB:   492#374642485936
						CitroenVinTypeAsegC:   4b2#484a373334323133

			CitroenVinTypeB:
					Models: Citroën Jumper 2018 (ex. VF7YA1MFB12H99607)
							//ex. "autoPiUnitId": "b4cc83c4-10ef-7b36-d88c-71258deefe83",
					Protocol: 7
					Sample VIN data:
						CitroenVinTypeBsegA:   0814C201#005646375941314D
						CitroenVinTypeBsegB:   0814C201#0146423132483939
						CitroenVinTypeBsegC:   0814C201#0236303700000000
*/
