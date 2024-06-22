package internal

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/DIMO-Network/edge-network/internal/api"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/rs/zerolog"
)

type keyType string

const LogToMqtt keyType = "mqtt_log"
const StopLogAfter keyType = "stopLogAfter"
const ThresholdWhenLogMqtt keyType = "threshold"
const PowerStatus keyType = "powerStatus"

type LogHook struct {
	DataSender network.DataSender
}

// Run All fatal log level will be sent to mqtt,
// if we want error log level be sent to mqtt bus, we need to log it like:
// logger.Error().Ctx(context.WithValue(context.Background(), internal.LogToMqtt, "true")).Msgf("Error msg: %s", err.Error())
func (h *LogHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// This is where you can modify the event by adding fields, changing
	// existing ones, or even decide to not log the event.
	if level == zerolog.FatalLevel {
		err := h.DataSender.SendErrorPayload(errors.New(msg), nil)
		if err != nil {
			return
		}
	}

	if e.GetCtx().Value(LogToMqtt) != nil {
		err := h.DataSender.SendErrorPayload(errors.New(msg), nil)
		if err != nil {
			return
		}
		e.Str(string(LogToMqtt), "send message over the mqtt bus")
	}
}

// LogRateLimiterHook is a log hook that filters log events based on the number of times an error message has occurred.
// The hook keeps track of the number of times an error message has occurred and sends the error payload to MQTT when the count reaches a certain threshold.
type LogRateLimiterHook struct {
	errorCounts map[string]int
	DataSender  network.DataSender
	mu          sync.Mutex
}

func NewLogRateLimiterHook(dataSender network.DataSender) *LogRateLimiterHook {
	return &LogRateLimiterHook{
		errorCounts: make(map[string]int),
		DataSender:  dataSender,
	}
}

// Run processes the log event based on the provided context values.
// If the context contains a "stopLogAfter" or "threshold" value, the function will increment the count for the error message in the errorCounts map.
// If "stopLogAfter" is set and the count for the error message exceeds this value, the log event is discarded.
// If "threshold" is set and the count for the error message reaches this value, the function sends the error payload to MQTT and resets the count for the error message in the errorCounts map.
// If neither "stopLogAfter" nor "threshold" is set in the context, the function will not filter the log event.
func (h *LogRateLimiterHook) Run(e *zerolog.Event, _ zerolog.Level, msg string) {
	// If the log level is error, increment the count for the error
	stopLogAfter, okStopLogAfter := e.GetCtx().Value(StopLogAfter).(int)
	threshold, okThreshold := e.GetCtx().Value(ThresholdWhenLogMqtt).(int)
	powerStatus, okPowerStatus := e.GetCtx().Value(PowerStatus).(api.PowerStatusResponse)
	if (okThreshold && threshold > 0) || (okStopLogAfter && stopLogAfter > 0) || okPowerStatus {

		// If the threshold is less than or equal to 0, never send to MQTT
		if threshold <= 0 {
			threshold = math.MaxInt32
		}

		// If the stopLogAfter value is less than or equal to 0, set it to 5
		if stopLogAfter <= 0 {
			stopLogAfter = 5
		}

		h.mu.Lock()
		h.errorCounts[msg]++
		count := h.errorCounts[msg]
		h.mu.Unlock()

		// If the error has occurred a number of times equal to the stopLogAfter value, discard the log event
		if count > stopLogAfter {
			e.Discard()
		}

		// If the error has occurred a number of times equal to the threshold, send the error payload to MQTT and reset the count
		if count >= threshold {
			err := h.DataSender.SendErrorPayload(errors.New(msg), &powerStatus)
			if err != nil {
				return
			}
			h.mu.Lock()
			h.errorCounts[msg] = 0
			h.mu.Unlock()
		}
	}
}

type logOption func(*logOptions)

// logOptions contains the options for the logError function.
type logOptions struct {
	stopLogAfter         *int
	thresholdWhenLogMqtt *int
	powerStatus          *api.PowerStatusResponse
}

func withStopLogAfter(stopLogAfter int) logOption {
	return func(o *logOptions) {
		o.stopLogAfter = &stopLogAfter
	}
}

func withThresholdWhenLogMqtt(thresholdWhenLogMqtt int) logOption {
	return func(o *logOptions) {
		o.thresholdWhenLogMqtt = &thresholdWhenLogMqtt
	}
}

func withPowerStatus(powerStatus api.PowerStatusResponse) logOption {
	return func(o *logOptions) {
		o.powerStatus = &powerStatus
	}
}

// logError logs an error message with the provided options.
func logError(logger zerolog.Logger, err error, message string, opts ...logOption) {
	c := applyOptions(opts)

	logger.Err(err).Ctx(c).Msg(message)
}

// logInfo logs an info message with the provided options.
func logInfo(logger zerolog.Logger, message string, opts ...logOption) {
	c := applyOptions(opts)

	logger.Info().Ctx(c).Msg(message)
}

func applyOptions(opts []logOption) context.Context {
	options := &logOptions{}
	for _, opt := range opts {
		opt(options)
	}
	c := context.Background()
	if options.stopLogAfter != nil {
		c = context.WithValue(c, StopLogAfter, *options.stopLogAfter)
	}
	if options.thresholdWhenLogMqtt != nil {
		c = context.WithValue(c, ThresholdWhenLogMqtt, *options.thresholdWhenLogMqtt)
	}
	if options.powerStatus != nil {
		c = context.WithValue(c, PowerStatus, *options.powerStatus)
	}
	return c
}
