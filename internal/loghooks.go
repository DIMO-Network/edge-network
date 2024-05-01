package internal

import (
	"errors"
	"github.com/DIMO-Network/edge-network/internal/network"
	"github.com/rs/zerolog"
)

type keyType string

const LogToMqtt keyType = "mqtt_log"

type LogHook struct {
	DataSender network.DataSender
}

// Run All fatal log level will be sent to mqtt,
// if we want error log level be sent to mqtt bus, we need to log it like:
// logger.Err(err).Ctx(context.WithValue(context.Background(), "mqtt", "true")).Msgf("Error msg: %s", err)
func (h *LogHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// This is where you can modify the event by adding fields, changing
	// existing ones, or even decide to not log the event.
	if level == zerolog.FatalLevel {
		err := h.DataSender.SendErrorPayload(errors.New(msg), nil)
		if err != nil {
			return
		}
	}

	if level == zerolog.ErrorLevel {
		if e.GetCtx().Value(LogToMqtt) != nil {
			err := h.DataSender.SendErrorPayload(errors.New(msg), nil)
			if err != nil {
				return
			}
			e.Str(string(LogToMqtt), "send message over the mqtt bus")
		}
	}
}
