package internal

import (
	"context"
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

type FunctionWithError[T any] func() (*T, error)

// FunctionWithFailureHandler is a function that will log an error message only once and  if the failure threshold is reached
type FunctionWithFailureHandler[T any] func(fn FunctionWithError[T]) (*T, error)

// NewFailureHandler returns a function that will log an error message only once and  if the failure threshold is reached
// it will send an error to mqtt
func NewFailureHandler[T any](logger zerolog.Logger, failureThreshold int, errorMessage string) FunctionWithFailureHandler[T] {
	var hasLoggedFailure bool
	var failureCount int

	return func(fn FunctionWithError[T]) (*T, error) {
		result, err := fn()
		if err != nil {
			if !hasLoggedFailure {
				logger.Err(err).Msg(errorMessage)
				hasLoggedFailure = true
			}

			failureCount++
			logger.Info().Msgf("failure count for %s: %d", errorMessage, failureCount)
			if failureCount >= failureThreshold {
				logger.Err(err).Ctx(context.WithValue(context.Background(), LogToMqtt, "true")).
					Msgf(errorMessage+" %d times in a row", failureCount)
				failureCount = 0
			}

			return nil, err
		}

		hasLoggedFailure = false
		failureCount = 0

		return result, nil
	}
}
