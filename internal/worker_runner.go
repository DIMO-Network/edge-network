package internal

import (
	"fmt"
	"strconv"
	"sync"
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
	unitID              uuid.UUID
	loggerSettingsSvc   loggers.TemplateStore
	queueSvc            queue.StorageQueue
	dataSender          network.DataSender
	logger              zerolog.Logger
	ethAddr             *common.Address
	fingerprintRunner   FingerprintRunner
	pids                *models.TemplatePIDs
	deviceSettings      *models.TemplateDeviceSettings
	signalsQueue        *SignalsQueue
	stop                chan bool
	sendPayloadInterval time.Duration
}

func NewWorkerRunner(unitID uuid.UUID, addr *common.Address, loggerSettingsSvc loggers.TemplateStore,
	queueSvc queue.StorageQueue, dataSender network.DataSender, logger zerolog.Logger, fpRunner FingerprintRunner, pids *models.TemplatePIDs, settings *models.TemplateDeviceSettings) WorkerRunner {
	signalsQueue := &SignalsQueue{lastTimeChecked: make(map[string]time.Time)}
	// Interval for sending status payload to cloud. Status payload contains obd signals and non-obd signals.
	interval := 20 * time.Second
	return &workerRunner{unitID: unitID, ethAddr: addr, loggerSettingsSvc: loggerSettingsSvc,
		queueSvc: queueSvc, dataSender: dataSender, logger: logger, fingerprintRunner: fpRunner, pids: pids, deviceSettings: settings, signalsQueue: signalsQueue, sendPayloadInterval: interval}
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
	// battery-voltage will be checked in obd related clock to determine if it is ok to query obd
	// battery-voltage also will be checked in non-obd clock because we want to send it with every status payload
	go func() {
		fingerprintDone := false
		for {
			// we will need to check the voltage before we query obd, and then we can query obd if voltage is ok
			queryOBD, powerStatus := wr.isOkToQueryOBD()
			if queryOBD {
				wr.logger.Debug().Msgf("voltage is enough to query obd : %f\n", powerStatus.VoltageFound)
				// do fingerprint but only once
				if !fingerprintDone {
					err := wr.fingerprintRunner.FingerprintSimple(powerStatus)
					if err != nil {
						wr.logger.Err(err).Msg("failed to do vehicle fingerprint")
					} else {
						fingerprintDone = true
					}
				}
				// query OBD signals
				wr.queryOBD()
			} else {
				wr.logger.Info().Msg("voltage not enough to query obd")
			}

			time.Sleep(2 * time.Second)
		}
	}()

	// Note: this delay required for the tests only, to make sure that queryOBD is executed before nonObd signals
	time.Sleep(1 * time.Second)
	for {
		select {
		case <-wr.stop:
			// If stop signal is received, stop the loop
			// Note: this is used only for unit/functional tests
			fmt.Println("Stopping worker runner")
			return
		default:
			_, powerStatus := wr.isOkToQueryOBD()
			// query non-obd signals even if voltage is not enough
			wifi, wifiErr, location, locationErr, cellInfo, cellErr := wr.queryNonObd(modem)
			// compose the device event
			s := wr.composeDeviceEvent(powerStatus, locationErr, location, wifiErr, wifi, cellErr, cellInfo)

			// send the cloud event
			err = wr.dataSender.SendDeviceStatusData(s)
			if err != nil {
				wr.logger.Err(err).Msg("failed to send device status in loop")
			}

			// todo: maybe we should send the location more frequently, maybe every 10 seconds
			time.Sleep(wr.sendPayloadInterval)
		}
	}
}

// Stop is used only for functional tests
func (wr *workerRunner) Stop() {
	wr.stop <- true
}

func (wr *workerRunner) composeDeviceEvent(powerStatus api.PowerStatusResponse, locationErr error, location *models.Location, wifiErr error, wifi *models.WiFi, cellErr error, cellInfo api.QMICellInfoResponse) models.DeviceStatusData {
	statusData := models.DeviceStatusData{
		CommonData: models.CommonData{
			Timestamp: time.Now().UTC().UnixMilli(),
		},
		Device: models.Device{
			RpiUptimeSecs:  powerStatus.Rpi.Uptime.Seconds,
			BatteryVoltage: powerStatus.VoltageFound,
		},
		Vehicle: models.Vehicle{
			Signals: wr.signalsQueue.Dequeue(),
		},
	}
	// only update location if no error
	if locationErr == nil {
		statusData.Location = location
	}
	n := &models.Network{}

	// only update Wi-Fi if no error
	if wifiErr == nil {
		n.WiFi = *wifi
		statusData.Network = n
	}
	// only update cell info if no error
	if cellErr == nil {
		n.QMICellInfoResponse = cellInfo
		statusData.Network = n
	}

	return statusData
}

func (wr *workerRunner) queryNonObd(modem string) (*models.WiFi, error, *models.Location, error, api.QMICellInfoResponse, error) {
	wifi, wifiErr := wr.queryWiFi()
	location, locationErr := wr.queryLocation(modem)
	cellInfo, cellErr := commands.GetQMICellInfo(wr.unitID)
	if cellErr != nil {
		wr.logger.Err(cellErr).Msg("failed to get qmi cell info")
	}
	return wifi, wifiErr, location, locationErr, cellInfo, cellErr
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

func (wr *workerRunner) queryOBD() {
	for _, request := range wr.pids.Requests {
		// check if ok to query this pid
		if lastEnqueuedTime, ok := wr.signalsQueue.lastEnqueuedTime(request.Name); ok {
			// if interval is 0, then we only query once at the device startup
			if request.IntervalSeconds == 0 {
				continue
			}
			if int(time.Since(lastEnqueuedTime).Seconds()) < request.IntervalSeconds {
				continue
			}
		}

		protocol, err := strconv.Atoi(request.Protocol)
		if err != nil {
			protocol = 6
		}
		pidStr := uintToHexStr(request.Pid)
		hexResp, ts, err := commands.RequestPIDRaw(wr.unitID, request.Name, fmt.Sprintf("%X", request.Header), uintToHexStr(request.Mode),
			pidStr, protocol, request)
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
			wr.signalsQueue.Enqueue(models.SignalData{
				Timestamp: ts.UnixMilli(),
				Name:      request.Name,
				Value:     value,
			})
		} else {
			wr.logger.Error().Msgf("no recognized formula type found: %s", request.Formula)
		}
	}

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

type SignalsQueue struct {
	signals         []models.SignalData
	lastTimeChecked map[string]time.Time
	sync.RWMutex
}

func (sq *SignalsQueue) lastEnqueuedTime(key string) (time.Time, bool) {
	sq.Lock()
	defer sq.Unlock()
	t, ok := sq.lastTimeChecked[key]
	return t, ok

}

func (sq *SignalsQueue) Enqueue(signal models.SignalData) {
	sq.Lock()
	defer sq.Unlock()
	sq.lastTimeChecked[signal.Name] = time.Now()
	sq.signals = append(sq.signals, signal)
}

func (sq *SignalsQueue) Dequeue() []models.SignalData {
	sq.Lock()
	defer sq.Unlock()
	signals := sq.signals
	// empty the data after dequeue
	sq.signals = []models.SignalData{}
	return signals
}
