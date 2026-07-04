// Package cbreaker cb.go contains methods for
// create circuit breaker
package cbreaker

import (
	"os"
	"strconv"
	"time"

	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
)

// getEnvInt returns the value of the environment variable
// or the default value if the environment variable is not set
func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// NewCB creates a new circuit breaker
// with values from environment variables
func NewCB(name string, log *zap.Logger) *gobreaker.CircuitBreaker[any] {
	const op = "cbreaker.NewCB"

	maxRequests := uint32(getEnvInt("CBMaxRequests", 5))
	timeout := time.Duration(getEnvInt("CBTimeout", 20))
	interval := time.Duration(getEnvInt("CBInterval", 30))
	failCnt := uint32(getEnvInt("CBFailCnt", 5))

	st := gobreaker.Settings{
		Name:        name,
		MaxRequests: maxRequests,
		Timeout:     timeout * time.Second,
		Interval:    interval * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= failCnt
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
				zap.String("op", op))
		},
	}
	return gobreaker.NewCircuitBreaker[any](st)
}
