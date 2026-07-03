// Package middlewares contains middlewares
package middlewares

import "context"

type Middleware interface {
	Handle(ctx context.Context) error
}

type RateLimiter struct{}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{}
}

func (rl *RateLimiter) Handle(ctx context.Context) error {
	return nil
}
