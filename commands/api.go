package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	detectCanbusCommand        = `obd.protocol set=auto`
	sleepTimerDelayCommand     = `power.sleep_timer add=pairing period=900 clear=*`
	getEthereumAddressCommand  = `crypto.query ethereum_address`
	signHashCommand            = `crypto.sign_string `
	getDeviceIDCommand         = `config.get device.id`
	getHardwareRevisionCommand = `config.get hw.version`
	signalStrengthCommand      = `qmi.signal_strength`
	wifiStatusCommand          = `wifi.status`
	setWifiConnectionCommand   = `grains.set`
	getSoftwareVersionCommand  = `grains.get release:version`
	getDiagnosticCodeCommand   = `obd.dtc`
	clearDiagnosticCodeCommand = `obd.dtc clear=true`
	powerStatusCommand         = `power.status`
)

const autoPiBaseURL = "http://192.168.4.1:9000"
const contentTypeJSON = "application/json"

type KwargType struct {
	Destructive bool `json:"destructive,omitempty"`
	Force       bool `json:"force,omitempty"`
}
type executeRawRequest struct {
	Command string        `json:"command"`
	Arg     []interface{} `json:"arg"`
	Kwarg   KwargType     `json:"kwarg"`
}

// For some reason, this only gets returned for some calls.
type executeRawResponse struct {
	Value string `json:"value"`
}

type GenericSignalStrengthResponse struct {
	Network string  `json:"network"`
	Unit    string  `json:"unit"`
	Value   float64 `json:"value"`
}

type signalStrengthResponse struct {
	Current GenericSignalStrengthResponse
}

type wifiConnectionsResponse struct {
	WPAState string `json:"wpa_state"`
	SSID     string `json:"ssid"`
}

type WifiEntity struct {
	Priority int    `json:"priority"`
	Psk      string `json:"psk"`
	SSID     string `json:"ssid"`
}

type setWifiConnectionResponse struct {
	Comment string `json:"comment"`
	Result  bool   `json:"result"`
	Changes struct {
		WPASupplicant struct {
			Networks []WifiEntity
		} `json:"wpa_supplicant"`
	}
}

type SetWifiRequest struct {
	Network  string `json:"network"`
	Password string `json:"password"`
}

type dtcResponse struct {
	Stamp  string `json:"_stamp"`
	Type   string `json:"_type"`
	Values []struct {
		Code string `json:"code"`
		Text string `json:"text"`
	} `json:"values"`
}

type CanbusInfo struct {
	Autodetected bool   `json:"autodetected"`
	Baudrate     int    `json:"baudrate"`
	Ecus         []int  `json:"ecus"`
	ID           string `json:"id"`
	Name         string `json:"name"`
}

type obdAutoDetectResponse struct {
	Stamp      string     `json:"_stamp"`
	CanbusInfo CanbusInfo `json:"current"`
}

type powerStatusResponse struct {
	Rpi struct {
		Uptime struct {
			Days     int    `json:"days"`
			Seconds  int    `json:"seconds"`
			SinceIso string `json:"since_iso"`
			SinceT   int    `json:"since_t"`
			Time     string `json:"time"`
			Users    int    `json:"users"`
		} `json:"uptime"`
	} `json:"rpi"`
	Spm struct {
		Battery struct {
			Level   int     `json:"level"`
			State   string  `json:"state"`
			Voltage float64 `json:"voltage"`
		} `json:"battery"`
		CurrentState string `json:"current_state"`
		LastState    struct {
			Down string `json:"down"`
			Up   string `json:"up"`
		} `json:"last_state"`
		LastTrigger struct {
			Down string `json:"down"`
			Up   string `json:"up"`
		} `json:"last_trigger"`
		SleepInterval int     `json:"sleep_interval"`
		Version       string  `json:"version"`
		VoltFactor    float64 `json:"volt_factor"`
		VoltTriggers  struct {
			HibernateLevel struct {
				Duration  int     `json:"duration"`
				Threshold float64 `json:"threshold"`
			} `json:"hibernate_level"`
			WakeChange struct {
				Difference float64 `json:"difference"`
				Period     int     `json:"period"`
			} `json:"wake_change"`
			WakeLevel struct {
				Duration  int     `json:"duration"`
				Threshold float64 `json:"threshold"`
			} `json:"wake_level"`
		} `json:"volt_triggers"`
	} `json:"spm"`
}

func executeRequest(method, path string, reqVal, respVal any) (err error) {
	var reqBody io.Reader

	if reqVal != nil {
		reqBuf := new(bytes.Buffer)
		err = json.NewEncoder(reqBuf).Encode(reqVal)
		if err != nil {
			return
		}
		reqBody = reqBuf
	}

	req, err := http.NewRequest(method, autoPiBaseURL+path, reqBody)
	if err != nil {
		return
	}

	if reqVal != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if c := resp.StatusCode; c >= 400 {
		return fmt.Errorf("status code %d", c)
	}

	if respVal == nil {
		return
	}
	err = json.NewDecoder(resp.Body).Decode(respVal)
	return
}
