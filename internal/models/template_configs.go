package models

import "strings"

// TemplateURLs holds the urls that point to different settings, version is in url
type TemplateURLs struct {
	DbcURL           string `json:"dbcUrl"`
	DeviceSettingURL string `json:"deviceSettingUrl"`
	PidURL           string `json:"pidUrl"`
}

// TemplatePIDs holds the pid requests we want to make to the vehicle
type TemplatePIDs struct {
	Requests     []PIDRequest `json:"requests"`
	TemplateName string       `json:"template_name"`
	Version      string       `json:"version"`
}

type PIDRequest struct {
	Formula              string `json:"formula"`
	Header               uint32 `json:"header"`
	IntervalSeconds      int    `json:"interval_seconds"`
	Mode                 uint32 `json:"mode"`
	Name                 string `json:"name"`
	Pid                  uint32 `json:"pid"`
	Protocol             string `json:"protocol"`
	CanflowControlClear  bool   `json:"can_flow_control_clear"`
	CanFlowControlIDPair string `json:"can_flow_control_id_pair"`
}

// FormulaType gets the first 3 chars of the formula which are expected to be the type, currently only know of dbc
func (p *PIDRequest) FormulaType() string {
	// asumming no multibyte characters
	if len(p.Formula) >= 3 {
		return p.Formula[:3]
	}
	return ""
}

// FormulaValue gets the formula without the 4 type characters at the beginning, eg. "dbc:"
func (p *PIDRequest) FormulaValue() string {
	return strings.TrimSpace(p.Formula[4:])
}

// TemplateDeviceSettings contains configurations options around power and other device settings. share from: vehicle-signal-decoding.grpc.DeviceSetting
type TemplateDeviceSettings struct {
	BatteryCriticalLevelVoltage            float64 `json:"battery_critical_level_voltage"`
	SafetyCutOutVoltage                    float64 `json:"safety_cut_out_voltage"`
	SleepTimerEventDrivenInterval          float64 `json:"sleep_timer_event_driven_interval_secs"`
	SleepTimerEventDrivenPeriod            float64 `json:"sleep_timer_event_driven_period_secs"`
	SleepTimerInactivityAfterSleepInterval float64 `json:"sleep_timer_inactivity_after_sleep_interval_secs"`
	SleepTimerInactivityFallbackInterval   float64 `json:"sleep_timer_inactivity_fallback_interval_secs"`
	TemplateName                           string  `json:"template_name"`
	WakeTriggerVoltageLevel                float64 `json:"wake_trigger_voltage_level"`
	MinVoltageOBDLoggers                   float64 `json:"min_voltage_obd_loggers"`
}

// VINLoggerSettings contains the settings we store locally related to the VIN (last VIN obtained and any other related info)
type VINLoggerSettings struct {
	// VIN is whatever VIN we last were able to get from the vehicle
	VIN                     string `json:"vin"`
	VINQueryName            string `json:"vin_query_name"`
	VINLoggerVersion        int    `json:"vin_logger_version"`
	VINLoggerFailedAttempts int    `json:"vin_logger_failed_attempts"`
}
