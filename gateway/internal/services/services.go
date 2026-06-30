// Package services contains interfaces for services
// with retry and circuit breaker functionality
package services

import (
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Closable interface {
	Close() error
	GetName() string
}

// CallRPC calls the function fn and retries it up to 5 times
// is uses circuit breaker to prevent overloading the server
func CallRPC[T any](cb *gobreaker.CircuitBreaker[any], fn func() (T, error)) (T, error) {
	const op = "services.CallRPC"

	var zero T

	resCb, err := cb.Execute(func() (any, error) {
		return retryRPC(func() (T, error) {
			return fn()
		})
	})
	if err != nil {
		return zero, fmt.Errorf("%s: rpc error %w", op, err)
	}

	res, ok := resCb.(T)
	if !ok {
		return zero, fmt.Errorf("%s: invalid response type", op)
	}

	return res, nil
}

// retryRPC retries the function fn up to 5 times
func retryRPC[T any](fn func() (T, error)) (T, error) {
	const op = "services.retryRPC"
	var zero T

	for i := range 5 {
		res, err := fn()
		if err == nil {
			return res, nil
		}
		if !shouldRetry(err) {
			return zero, err
		}

		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return zero, fmt.Errorf("%s: failed after 5 attempts", op)
}

// shouldRetry returns true if the error should be retried
func shouldRetry(err error) bool {
	st, ok := status.FromError(err)
	if ok {
		switch st.Code() {
		case
			codes.Canceled,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
			codes.Aborted,
			codes.Unavailable,
			codes.DataLoss:

			return true
		case
			codes.InvalidArgument,
			codes.NotFound,
			codes.AlreadyExists,
			codes.PermissionDenied,
			codes.FailedPrecondition,
			codes.OutOfRange,
			codes.Unimplemented,
			codes.Internal,
			codes.Unauthenticated:

			return false
		}
	}

	return false
}
