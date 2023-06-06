package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	DetectCanbusCommand        = `obd.protocol set=auto`
	SleepTimerDelayCommand     = `power.sleep_timer add=pairing period=900 clear=*`
	GetEthereumAddressCommand  = `crypto.query ethereum_address`
	SignHashCommand            = `crypto.sign_string `
	GetDeviceIDCommand         = `config.get device.id`
	GetHardwareRevisionCommand = `config.get hw.version`
	SignalStrengthCommand      = `qmi.signal_strength`
	WifiStatusCommand          = `wifi.status`
	SetWifiConnectionCommand   = `grains.set`
	GetSoftwareVersionCommand  = `config.get latest_release_version`
	GetDiagnosticCodeCommand   = `obd.dtc`
	ClearDiagnosticCodeCommand = `obd.dtc clear=true`
	PowerStatusCommand         = `power.status`
	GetModemCommand            = `config.get modem`
	Ec2xIMSICommand            = `ec2x.query AT+CIMI`
	NormalIMSICommand          = `modem.connection execute AT+CIMI`
)

const autoPiBaseURL = "http://192.168.4.1:9000"
const contentTypeJSON = "application/json"

type KwargType struct {
	Destructive bool `json:"destructive,omitempty"`
	Force       bool `json:"force,omitempty"`
}
type ExecuteRawRequest struct {
	Command string        `json:"command"`
	Arg     []interface{} `json:"arg"`
	Kwarg   KwargType     `json:"kwarg"`
}

// For some reason, this only gets returned for some calls.
// Sometimes it's "value", sometimes "data".
type ExecuteRawResponse struct {
	Value string `json:"value"`
	Data  string `json:"data"`
}

type GenericSignalStrengthResponse struct {
	Network string  `json:"network"`
	Unit    string  `json:"unit"`
	Value   float64 `json:"value"`
}

type SignalStrengthResponse struct {
	Current GenericSignalStrengthResponse
}

type WifiConnectionsResponse struct {
	WPAState string `json:"wpa_state"`
	SSID     string `json:"ssid"`
}

type WifiEntity struct {
	Priority int    `json:"priority"`
	Psk      string `json:"psk"`
	SSID     string `json:"ssid"`
}

type SetWifiConnectionResponse struct {
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

type DTCResponse struct {
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

type ObdAutoDetectResponse struct {
	Stamp      string     `json:"_stamp"`
	CanbusInfo CanbusInfo `json:"current"`
}

type PowerStatusResponse struct {
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

func ExecuteRequest(method, path string, reqVal, respVal any) (err error) {
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
		body, _ := io.ReadAll(resp.Body)
		if body == nil {
			body = []byte("no body response.")
		}
		return fmt.Errorf("status code %d. body: %s", c, string(body))
	}

	// Ignore any response.
	if respVal == nil {
		return
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(b, respVal)
	return
}
