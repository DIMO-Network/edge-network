package internal

import (
	"errors"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/rs/zerolog"
	"sync"
)

type keyType string

const LogToMqtt keyType = "mqtt_log"

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

// FilterHook is a log hook that filters error logs and sends them to MQTT
type FilterHook struct {
	errorCounts map[string]int
	DataSender  network.DataSender
	mu          sync.Mutex
}

func NewFilterHook(dataSender network.DataSender) *FilterHook {
	return &FilterHook{
		errorCounts: make(map[string]int),
		DataSender:  dataSender,
	}
}

//	Run gets the threshold from the context with e.GetCtx().Value("threshold").(int)
//
// It increments the count for the error message in the errorCounts map.
// If the threshold is not set in the context, we skip the filter.
// If the count for the error message reaches the threshold, it sends the error payload to MQTT
// and resets the count for the error message in the errorCounts map.
func (h *FilterHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// If the log level is error, increment the count for the error
	threshold, ok := e.GetCtx().Value("threshold").(int)
	if ok && threshold > 0 {
		h.mu.Lock()
		h.errorCounts[msg]++
		count := h.errorCounts[msg]
		h.mu.Unlock()

		// If the error has occurred for the first time, log it
		if count == 1 {
			e.Msg(msg)
		}

		// Get the threshold from the context
		threshold, ok := e.GetCtx().Value("threshold").(int)
		if !ok {
			// If the threshold is not set in the context, use a default value
			threshold = 10
		}

		// If the error has occurred a number of times equal to the threshold, send the error payload to MQTT and reset the count
		if count >= threshold {
			err := h.DataSender.SendErrorPayload(errors.New(msg), nil)
			if err != nil {
				return
			}
			h.mu.Lock()
			h.errorCounts[msg] = 0
			h.mu.Unlock()
		}
	}
}
