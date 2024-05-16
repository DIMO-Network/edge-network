package internal

import (
	"fmt"
	"sync"
	"time"

	"github.com/DIMO-Network/edge-network/commands"
	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/loggers"
	"github.com/DIMO-Network/edge-network/internal/models"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type WorkerRunner interface {
	Run()
}

// Device represents the device information that is used in the worker runner
// to construct the device event
type Device struct {
	SoftwareVersion string
	HardwareVersion string
	IMEI            string
	UnitID          uuid.UUID
}

type workerRunner struct {
	loggerSettingsSvc   loggers.TemplateStore
	dataSender          network.DataSender
	logger              zerolog.Logger
	ethAddr             *common.Address
	fingerprintRunner   FingerprintRunner
	pids                *models.TemplatePIDs
	deviceSettings      *models.TemplateDeviceSettings
	signalsQueue        *SignalsQueue
	stop                chan bool
	sendPayloadInterval time.Duration
	device              Device
}

func NewWorkerRunner(addr *common.Address, loggerSettingsSvc loggers.TemplateStore,
	dataSender network.DataSender, logger zerolog.Logger, fpRunner FingerprintRunner,
	pids *models.TemplatePIDs, settings *models.TemplateDeviceSettings, device Device) WorkerRunner {
	signalsQueue := &SignalsQueue{lastTimeChecked: make(map[string]time.Time)}
	// Interval for sending status payload to cloud. Status payload contains obd signals and non-obd signals.
	interval := 20 * time.Second
	return &workerRunner{ethAddr: addr, loggerSettingsSvc: loggerSettingsSvc,
		dataSender: dataSender, logger: logger, fingerprintRunner: fpRunner, pids: pids, deviceSettings: settings, signalsQueue: signalsQueue, sendPayloadInterval: interval, device: device}
}

// Run sends a signed status payload every X seconds, that may or may not contain OBD signals.
// It also has a continuous loop that checks voltage compared to template settings to make sure ok to query OBD.
// It will query the VIN once on startup and send a fingerprint payload (only once per Run).
// If ok to query OBD, queries each signal per it's designated interval.
func (wr *workerRunner) Run() {
	// requires deviceSettings and pids (even if empty) to run
	vin, err := wr.loggerSettingsSvc.ReadVINConfig() // this could return nil vin
	if err != nil {
		wr.logger.Err(err).Msg("unable to get vin for worker runner from past state, continuing")
	}
	if vin != nil {
		wr.logger.Info().Msgf("starting worker runner with vin: %s", vin.VIN)
	}
	wr.logger.Info().Msgf("starting worker runner with logger settings: %+v", wr.deviceSettings)

	modem, err := commands.GetModemType(wr.device.UnitID)
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

	// start the location query if the frequency is set
	// float e.g. 0.5 would be 2x per second
	// do not start the location query if the frequency is 0 or sendPayloadInterval (which is 20s)
	if wr.deviceSettings.LocationFrequencySecs > 0 && wr.deviceSettings.LocationFrequencySecs != wr.sendPayloadInterval.Seconds() {
		wr.startLocationQuery(modem)
	}

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
			s := wr.composeDeviceEvent(powerStatus, locationErr, location, wifiErr, wifi)

			// send the cloud event
			err = wr.dataSender.SendDeviceStatusData(s)
			if err != nil {
				wr.logger.Err(err).Msg("failed to send device status in loop")
			}

			if cellErr == nil || wifiErr == nil {
				// compose the device network event
				networkData := models.DeviceNetworkData{
					CommonData: models.CommonData{
						Timestamp: time.Now().UTC().UnixMilli(),
					},
				}
				if locationErr == nil {
					networkData.Altitude = location.Altitude
					networkData.Hdop = location.Hdop
					networkData.Nsat = location.Nsat
					networkData.Latitude = location.Latitude
					networkData.Longitude = location.Longitude
				}
				if cellErr == nil {
					networkData.Cell = models.CellInfo{
						Details: cellInfo.IntrafrequencyLteInfo,
					}
				}

				err = wr.dataSender.SendDeviceNetworkData(networkData)
				if err != nil {
					wr.logger.Err(err).Msg("failed to send device network data")
				}
			}

			// future: send only the location more frequently, every 10 seconds ?
			time.Sleep(wr.sendPayloadInterval)
		}
	}
}

func (wr *workerRunner) startLocationQuery(modem string) {
	go func() {
		wr.logger.Info().Msgf("Start query location data with every %.2f sec", wr.deviceSettings.LocationFrequencySecs)
		for {
			location, locationErr := wr.queryLocation(modem)
			if locationErr == nil {

				wr.signalsQueue.Enqueue(models.SignalData{
					Timestamp: time.Now().UTC().UnixMilli(),
					Name:      "longitude",
					Value:     location.Longitude,
				})

				wr.signalsQueue.Enqueue(models.SignalData{
					Timestamp: time.Now().UTC().UnixMilli(),
					Name:      "latitude",
					Value:     location.Latitude,
				})

				wr.signalsQueue.Enqueue(models.SignalData{
					Timestamp: time.Now().UTC().UnixMilli(),
					Name:      "hdop",
					Value:     location.Hdop,
				})

				wr.signalsQueue.Enqueue(models.SignalData{
					Timestamp: time.Now().UTC().UnixMilli(),
					Name:      "nsat",
					Value:     location.Nsat,
				})

				wr.signalsQueue.Enqueue(models.SignalData{
					Timestamp: time.Now().UTC().UnixMilli(),
					Name:      "altitude",
					Value:     location.Altitude,
				})
				wr.logger.Debug().Msg("location data sent")
			}
			// convert float seconds to int nanoseconds
			intNanoseconds := int(wr.deviceSettings.LocationFrequencySecs * 1e9)
			time.Sleep(time.Duration(intNanoseconds))
		}
	}()
}

// Stop is used only for functional tests
func (wr *workerRunner) Stop() {
	wr.stop <- true
}

func (wr *workerRunner) composeDeviceEvent(powerStatus api.PowerStatusResponse, locationErr error, location *models.Location, wifiErr error, wifi *models.WiFi) models.DeviceStatusData {
	statusData := models.DeviceStatusData{
		CommonData: models.CommonData{
			Timestamp: time.Now().UTC().UnixMilli(),
		},
		Device: models.Device{
			RpiUptimeSecs:   powerStatus.Rpi.Uptime.Seconds,
			BatteryVoltage:  powerStatus.VoltageFound,
			SoftwareVersion: wr.device.SoftwareVersion,
			HardwareVersion: wr.device.HardwareVersion,
			UnitID:          wr.device.UnitID.String(),
			IMEI:            wr.device.IMEI,
		},
		Vehicle: models.Vehicle{
			Signals: wr.signalsQueue.Dequeue(),
		},
	}
	// only update location if no error
	if locationErr == nil {
		statusData.Vehicle.Signals = appendSignalData(statusData.Vehicle.Signals, "longitude", location.Longitude)
		statusData.Vehicle.Signals = appendSignalData(statusData.Vehicle.Signals, "latitude", location.Latitude)
		statusData.Vehicle.Signals = appendSignalData(statusData.Vehicle.Signals, "hdop", location.Hdop)
		statusData.Vehicle.Signals = appendSignalData(statusData.Vehicle.Signals, "nsat", location.Nsat)
		statusData.Vehicle.Signals = appendSignalData(statusData.Vehicle.Signals, "altitude", location.Altitude)
	}

	// only update Wi-Fi if no error
	if wifiErr == nil {
		statusData.Vehicle.Signals = appendSignalData(statusData.Vehicle.Signals, "wpa_state", wifi.WPAState)
		statusData.Vehicle.Signals = appendSignalData(statusData.Vehicle.Signals, "ssid", wifi.SSID)
	}

	return statusData
}

func appendSignalData(signals []models.SignalData, name string, value interface{}) []models.SignalData {
	return append(signals, models.SignalData{
		Timestamp: time.Now().UTC().UnixMilli(),
		Name:      name,
		Value:     value,
	})
}

func (wr *workerRunner) queryNonObd(modem string) (*models.WiFi, error, *models.Location, error, api.QMICellInfoResponse, error) {
	wifi, wifiErr := wr.queryWiFi()
	location, locationErr := wr.queryLocation(modem)
	cellInfo, cellErr := commands.GetQMICellInfo(wr.device.UnitID)
	if cellErr != nil {
		wr.logger.Err(cellErr).Msg("failed to get qmi cell info")
	}
	return wifi, wifiErr, location, locationErr, cellInfo, cellErr
}

func (wr *workerRunner) queryWiFi() (*models.WiFi, error) {
	wifiStatus, err := commands.GetWifiStatus(wr.device.UnitID)
	wifi := models.WiFi{}
	if err != nil {
		wr.logger.Err(err).Msg("failed to get signal strength")
		return nil, err
	}
	wifi = models.WiFi{
		WPAState: wifiStatus.WPAState,
		SSID:     wifiStatus.SSID,
	}

	return &wifi, nil
}

func (wr *workerRunner) queryLocation(modem string) (*models.Location, error) {
	gspLocation, err := commands.GetGPSLocation(wr.device.UnitID, modem)
	location := models.Location{}
	if err != nil {
		wr.logger.Err(err).Msg("failed to get gps location")
		return nil, err
	}
	// location fields mapped to separate struct
	location = models.Location{
		Hdop:      gspLocation.Hdop,
		Nsat:      gspLocation.Nsat,
		Latitude:  gspLocation.Lat,
		Longitude: gspLocation.Lon,
		Altitude:  gspLocation.Alt,
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
		// execute the pid
		obdResp, ts, err := commands.RequestPIDRaw(&wr.logger, wr.device.UnitID, request)
		if err != nil {
			wr.logger.Err(err).Msg("failed to query obd pid")
			continue
		}
		// future: new formula type that could work for proprietary PIDs and could support text, int or float
		var value interface{}
		if request.FormulaType() == models.Dbc && obdResp.IsHex {
			value, _, err = loggers.ExtractAndDecodeWithDBCFormula(obdResp.ValueHex[0], uintToHexStr(request.Pid), request.FormulaValue())
			if err != nil {
				wr.logger.Err(err).Msgf("failed to convert hex response with formula. hex: %s", obdResp.ValueHex[0])
				continue
			}
		} else if !obdResp.IsHex {
			value = obdResp.Value
			// todo, check what other types conversion we should handle
		} else {
			wr.logger.Error().Msgf("no recognized formula type found: %s", request.Formula)
			continue
		}

		wr.signalsQueue.Enqueue(models.SignalData{
			Timestamp: ts.UnixMilli(),
			Name:      request.Name,
			Value:     value,
		})
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

// isOkToQueryOBD checks once to see if voltage rules pass to issue PID requests
func (wr *workerRunner) isOkToQueryOBD() (bool, api.PowerStatusResponse) {
	status, err := commands.GetPowerStatus(wr.device.UnitID)
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
