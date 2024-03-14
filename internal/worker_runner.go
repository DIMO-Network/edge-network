package internal

import (
	"fmt"
	"strconv"
	"time"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/DIMO-Network/edge-network/internal/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type WorkerRunner interface {
	Run()
}

type workerRunner struct {
	unitID            uuid.UUID
	loggerSettingsSvc loggers.TemplateStore
	queueSvc          queue.StorageQueue
	dataSender        network.DataSender
	logger            zerolog.Logger
	ethAddr           *common.Address
	fingerprintRunner FingerprintRunner
	pids              *models.TemplatePIDs
	deviceSettings    *models.TemplateDeviceSettings
}

func NewWorkerRunner(unitID uuid.UUID, addr *common.Address, loggerSettingsSvc loggers.TemplateStore,
	queueSvc queue.StorageQueue, dataSender network.DataSender, logger zerolog.Logger, fpRunner FingerprintRunner, pids *models.TemplatePIDs, settings *models.TemplateDeviceSettings) WorkerRunner {
	return &workerRunner{unitID: unitID, ethAddr: addr, loggerSettingsSvc: loggerSettingsSvc,
		queueSvc: queueSvc, dataSender: dataSender, logger: logger, fingerprintRunner: fpRunner, pids: pids, deviceSettings: settings}
}

// Run sends a signed status payload every X seconds, that may or may not contain OBD signals.
// It also has a continuous loop that checks voltage compared to template settings to make sure ok to query OBD.
// It will query the VIN once on startup and send a fingerprint payload (only once per Run).
// If ok to query OBD, queries each signal per it's designated interval.
func (wr *workerRunner) Run() {
	// todo v1: if no template settings obtained, we just want to send the status payload without obd stuff.

	vin, err := wr.loggerSettingsSvc.ReadVINConfig() // this could return nil vin
	if err != nil {
		wr.logger.Err(err).Msg("unable to get vin for worker runner from past state, continuing")
	}
	if vin != nil {
		wr.logger.Info().Msgf("starting worker runner with vin: %s", vin.VIN)
	}
	wr.logger.Info().Msgf("starting worker runner with logger settings: %+v", *wr.deviceSettings)

	modem, err := commands.GetModemType(wr.unitID)
	if err != nil {
		modem = "ec2x"
		wr.logger.Err(err).Msg("unable to get modem type, defaulting to ec2x")
	}
	wr.logger.Info().Msgf("found modem: %s", modem)

	// we will need two clocks, one for non-obd (every 20s) and one for obd (continuous, based on each signal interval)
	// which clock checks batteryvoltage? we want to send it with every status payload
	// register tasks that can be iterated over
	for {
		// naive implementation, just get messages sending - important to map out all actions. Later figure out right task engine
		queryOBD, powerStatus := wr.isOkToQueryOBD()
		// maybe start a timer here to know how long this cycle takes?
		// start a cloudevent. current loop without OBD takes about 2.2 seconds. We could parallelize these.
		signals := make([]models.SignalData, 1)
		// run through non obd ones: altitude, latitude, longitude, wifi connection, nsat (number gps satellites), cell signal info
		wifi, wifiErr := wr.queryWiFi()
		location, locationErr := wr.queryLocation(modem)
		cellInfo, cellErr := commands.GetQMICellInfo(wr.unitID)
		if cellErr != nil {
			wr.logger.Err(cellErr).Msg("failed to get qmi cell info")
		}

		// query OBD signals
		signals = wr.queryOBD(queryOBD, false, powerStatus, signals)

		// send the cloud event
		s := models.DeviceStatusData{
			CommonData: models.CommonData{
				Timestamp: time.Now().UTC().UnixMilli(),
			},
			Device: models.Device{
				RpiUptimeSecs:  powerStatus.Rpi.Uptime.Seconds,
				BatteryVoltage: powerStatus.VoltageFound,
			},
			Vehicle: models.Vehicle{
				Signals: signals,
			},
		}
		// only update location if no error
		if locationErr == nil {
			s.Location = location
		}
		n := &models.Network{}

		// only update wifi if no error
		if wifiErr == nil {
			n.WiFi = *wifi
			s.Network = n
		}
		// only update cell info if no error
		if cellErr == nil {
			n.QMICellInfoResponse = cellInfo
			s.Network = n
		}

		err = wr.dataSender.SendDeviceStatusData(s)
		if err != nil {
			wr.logger.Err(err).Msg("failed to send device status in loop")
		}
		time.Sleep(20 * time.Second)
	}

	//var tasks []WorkerTask
	//
	//pidTasks := wr.registerPIDsTasks(*wr.pids)
	//for _, task := range pidTasks {
	//	tasks = append(tasks, task)
	//}
	//
	//senderTasks := wr.registerSenderTasks()
	//for _, task := range senderTasks {
	//	tasks = append(tasks, task)
	//}
	//
	//var wg sync.WaitGroup
	//for i, task := range tasks {
	//	wg.Add(1)
	//	go func(idx int, t WorkerTask) {
	//		defer wg.Done()
	//		t.Execute(idx, wr.logger)
	//	}(i, task)
	//}
	//
	//wg.Wait()
	//wr.logger.Debug().Msg("worker Run completed")
}

func (wr *workerRunner) queryWiFi() (*models.WiFi, error) {
	wifiStatus, err := commands.GetWifiStatus(wr.unitID)
	wifi := models.WiFi{}
	if err != nil {
		wr.logger.Err(err).Msg("failed to get signal strength")
		return nil, err
	} else {
		wifi = models.WiFi{
			WPAState: wifiStatus.WPAState,
			SSID:     wifiStatus.SSID,
		}
	}
	return &wifi, nil
}

func (wr *workerRunner) queryLocation(modem string) (*models.Location, error) {
	gspLocation, err := commands.GetGPSLocation(wr.unitID, modem)
	location := models.Location{}
	if err != nil {
		wr.logger.Err(err).Msg("failed to get gps location")
		return nil, err
	} else {
		// location fields mapped to separate struct
		location = models.Location{
			Hdop:      gspLocation.Hdop,
			Nsat:      gspLocation.NsatGPS,
			Latitude:  gspLocation.Lat,
			Longitude: gspLocation.Lon,
		}
	}
	return &location, nil
}

func (wr *workerRunner) queryOBD(queryOBD bool, fingerprintDone bool, powerStatus api.PowerStatusResponse, signals []models.SignalData) []models.SignalData {
	if queryOBD {
		// do fingerprint but only once
		if !fingerprintDone {
			// todo we need to find a way how to mock it, currently I have issues to mock templateStore.ReadVINConfig()
			err := wr.fingerprintRunner.FingerprintSimple(powerStatus)
			if err != nil {
				wr.logger.Err(err).Msg("failed to do vehicle fingerprint")
			} else {
				fingerprintDone = true
			}
		}
		// run through all the obd pids (ignoring the interval? for now), maybe have a function that executes them from previously registered
		for _, request := range wr.pids.Requests {
			// todo: need a cache of when each pid was last called, to check for interval and see if ok to call again.
			protocol, err := strconv.Atoi(request.Protocol)
			if err != nil {
				protocol = 6
			}
			pidStr := uintToHexStr(request.Pid)
			hexResp, ts, err := commands.RequestPIDRaw(wr.unitID, request.Name, fmt.Sprintf("%X", request.Header), uintToHexStr(request.Mode),
				pidStr, protocol)
			if err != nil {
				wr.logger.Err(err).Msg("failed to query obd pid")
				continue
			}
			// todo new formula type that could work for proprietary PIDs and could support text, int or float
			if request.FormulaType() == "dbc" {
				value, _, err := loggers.ExtractAndDecodeWithDBCFormula(hexResp[0], pidStr, request.FormulaValue())
				if err != nil {
					wr.logger.Err(err).Msgf("failed to convert hex response with formula. hex: %s", hexResp[0])
					continue
				}
				signals = append(signals, models.SignalData{
					Timestamp: ts.UnixMilli(),
					Name:      request.Name,
					Value:     value,
				})
			} else {
				wr.logger.Error().Msgf("no recognized formula type found: %s", request.Formula)
			}
		}
	}
	return signals
}

// uintToHexStr converts the uint32 into a 0 padded hex representation, always assuming must be even length.
func uintToHexStr(val uint32) string {
	hexStr := fmt.Sprintf("%X", val)
	if len(hexStr)%2 != 0 {
		return "0" + hexStr // Prepend a "0" if the length is odd
	}
	return hexStr
}

func (wr *workerRunner) registerSenderTasks() []WorkerTask {
	var tasks []WorkerTask
	// build up task that sends mqtt payload every 60 seconds
	tasks = append(tasks, WorkerTask{
		Name:     "Sender Task",
		Interval: 60,
		Func: func(ctx WorkerTaskContext) {
			for {
				// are there many data points in one message? or is each message one signal data point
				messages, err := wr.queueSvc.Dequeue()
				if err != nil {
					wr.logger.Err(err).Msg("failed to Dequeue vehicle data signals")
					break
				}
				if len(messages) == 0 {
					break
				}
				signals := make([]models.SignalData, len(messages))
				for i, message := range messages {
					signals[i] = models.SignalData{Timestamp: message.Time.UnixMilli(), Name: message.Name, Value: message.Content}
				}
				err = wr.dataSender.SendDeviceStatusData(models.DeviceStatusData{
					Vehicle: models.Vehicle{
						Signals: signals,
					},
				})
				if err != nil {
					wr.logger.Err(err).Msg("Unable to send device status data")
				}
			}
		},
	})

	return tasks
}

func (wr *workerRunner) registerPIDsTasks(pidsConfig models.TemplatePIDs) []WorkerTask {

	if len(pidsConfig.Requests) > 0 {
		tasks := make([]WorkerTask, len(pidsConfig.Requests))

		for i, task := range pidsConfig.Requests {
			tasks[i] = WorkerTask{
				Name:     task.Name,
				Interval: task.IntervalSeconds,
				Once:     task.IntervalSeconds == 0,
				Params:   task,
				Func: func(wCtx WorkerTaskContext) {
					//err := wr.pidLog.ExecutePID(wCtx.Params.Header,
					//	wCtx.Params.Mode,
					//	wCtx.Params.Pid,
					//	wCtx.Params.Formula,
					//	wCtx.Params.Protocol,
					//	wCtx.Params.Name)
					//if err != nil {
					//	wr.logger.Err(err).Msg("failed execute pid loggers:" + wCtx.Params.Name)
					//}
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

		return tasks
	}

	return []WorkerTask{}
}

// isOkToQueryOBD checks once to see if voltage rules pass to issue PID requests
func (wr *workerRunner) isOkToQueryOBD() (bool, api.PowerStatusResponse) {
	status, err := commands.GetPowerStatus(wr.unitID)
	if err != nil {
		wr.logger.Err(err).Msg("failed to get powerStatus for worker runner check")
		return false, status
	}
	if status.VoltageFound >= wr.deviceSettings.MinVoltageOBDLoggers {
		return true, status
	}
	return false, status
}
