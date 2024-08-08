package util

import (
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// PointerType Define a type constraint that allows only pointer types.
type PointerType[T any] interface {
	*T // | // T can be any type as long as it's a pointer
	// *[]T | // T can be a slice pointer
	// *map[string]T // T can be a map pointer
}

// RetryableFunc This is the function type that we will retry
type RetryableFunc func() (interface{}, error)

// RetryableFuncErr This is the function type that we will retry that only returns an error
type RetryableFuncErr func() error

// Retry attempts to execute the provided function until it succeeds or the maximum number of attempts is reached.
// If the function returns an error that is of type Stop, it will not retry and will return the original error.
// It uses exponential backoff for the sleep duration between retries.
// Returns:
// *T - The result returned by the function, or nil if the function failed.
// error - The error returned by the function, or nil if the function succeeded.
func Retry[T any](attempts int, sleep time.Duration, logger zerolog.Logger, fn RetryableFunc) (*T, error) {
	var err error
	var result interface{}
	for i := 0; i < attempts; i++ {
		if result, err = fn(); err != nil {
			if _, ok := err.(Stop); ok {
				// Return the original error for later checking
				return nil, err
			}
			// Add some sleep here
			time.Sleep(sleep)
			sleep *= 2
		} else {
			if value, ok := result.(*T); ok {
				return value, nil
			}
			return nil, errors.New("type assertion failed")
		}
	}
	logger.Err(err).Msgf("Max retries reached for function")
	return nil, err
}

// RetryErrorOnly attempts to execute the provided function until it succeeds or the maximum number of attempts is reached.
// If the function returns an error that is of type Stop, it will not retry and will return the original error.
// It uses exponential backoff for the sleep duration between retries.
// Returns:
// error - The error returned by the function, or nil if the function succeeded.
func RetryErrorOnly(attempts int, sleep time.Duration, logger zerolog.Logger, fn RetryableFuncErr) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err != nil {
			if _, ok := err.(Stop); ok {
				// Return the original error for later checking
				return err
			}
			// Add some sleep here
			time.Sleep(sleep)
			sleep *= 2
		} else {
			return nil
		}
	}
	logger.Err(err).Msgf("Max retries reached for function")
	return err
}

// Stop is an error that wraps an error and is used to indicate that we should not retry
type Stop struct {
	Err error
}

func (s Stop) Error() string {
	return s.Err.Error()
}
