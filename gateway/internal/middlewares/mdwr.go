// Package middlewares contains middlewares
package middlewares

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/go-redis/redis/v8"
	tele "gopkg.in/telebot.v3"
)

// Middleware is an interface that defines a middleware
type Middleware interface {
	Handle(ctx context.Context, tctx tele.Context) error
}

// RateLimiter is a struct that implements Middleware interface
type RateLimiter struct {
	rdb        *redis.Client
	ctxTimeout time.Duration
	ttl        time.Duration
	limit      int
}

// NewRateLimiter creates new RateLimiter instance
func NewRateLimiter(redisCtxTimeout, rateLimitTTL, pingTimeout time.Duration) (*RateLimiter, error) {
	const op = "middlewares.NewRateLimiter"

	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_RL_ADDR"),
		Password: os.Getenv("REDIS_RL_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout*time.Second)
	defer cancel()

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("%s: ping: %w", op, err)
	}

	limit := os.Getenv("RATE_LIMIT")
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to parse rate limit: %w", op, err)
	}

	return &RateLimiter{
		rdb:        rdb,
		ctxTimeout: redisCtxTimeout,
		ttl:        rateLimitTTL,
		limit:      limitInt,
	}, nil
}

// incrAndExpire is a redis script that
// increments a key and sets an expiration atomically
var incrAndExpire = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return current
`)

// keyPool is a pool of byte slices
// for storing keys
var keyPool = sync.Pool{
	New: func() any {
		k := make([]byte, 0, 64)
		return &k
	},
}

// Handle implements Middleware interface
// It checks if the user has exceeded the rate limit
func (rl *RateLimiter) Handle(ctx context.Context, tctx tele.Context) error {
	const op = "middlewares.RateLimiter.Handle"

	keyPtr := keyPool.Get().(*[]byte)
	buf := (*keyPtr)[:0]
	defer keyPool.Put(keyPtr)

	buf = append(buf, "ratelimit:"...)
	buf = strconv.AppendInt(buf, tctx.Sender().ID, 10)
	rlKey := unsafe.String(unsafe.SliceData(buf), len(buf))

	count, err := incrAndExpire.Run(ctx, rl.rdb, []string{rlKey}, rl.ttl).Int()
	if err != nil {
		return fmt.Errorf("%s: failed to incr and expire: %w", op, err)
	}

	if count > rl.limit {
		return fmt.Errorf("%s: rate limit exceeded", op)
	}

	return nil
}
