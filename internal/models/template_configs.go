package models

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
	Formula         string `json:"formula"`
	Header          uint32 `json:"header"`
	IntervalSeconds int    `json:"interval_seconds"`
	Mode            uint32 `json:"mode"`
	Name            string `json:"name"`
	Pid             uint32 `json:"pid"`
	Protocol        string `json:"protocol"`
}

// TemplateDeviceSettings contains configurations options around power and other device settings
type TemplateDeviceSettings struct {
	BatteryCriticalLevelVoltage            string  `json:"battery_critical_level_voltage"`
	SafetyCutOutVoltage                    string  `json:"safety_cut_out_voltage"`
	SleepTimerEventDrivenInterval          string  `json:"sleep_timer_event_driven_interval_secs"`
	SleepTimerEventDrivenPeriod            string  `json:"sleep_timer_event_driven_period_secs"`
	SleepTimerInactivityAfterSleepInterval string  `json:"sleep_timer_inactivity_after_sleep_interval_secs"`
	SleepTimerInactivityFallbackInterval   string  `json:"sleep_timer_inactivity_fallback_interval_secs"`
	TemplateName                           string  `json:"template_name"`
	WakeTriggerVoltageLevel                string  `json:"wake_trigger_voltage_level"`
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
