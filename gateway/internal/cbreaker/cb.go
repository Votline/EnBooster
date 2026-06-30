// Pckage cbreaker cb.go contains methods for
// create circuit breaker
package cbreaker

import (
	"time"

	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
)

func NewCB(name string, log *zap.Logger) *gobreaker.CircuitBreaker[any] {
	const op = "cbreaker.NewCB"

	st := gobreaker.Settings{
		Name:        name,
		MaxRequests: 5,
		Timeout:     20 * time.Second,
		Interval:    30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
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
