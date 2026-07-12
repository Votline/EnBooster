// Package rdb provides Redis database connection
// and related methods.
package rdb

import (
	"context"
	"fmt"
	"os"
	"time"

	"aisrv/internal/utils"

	"github.com/go-redis/redis/v8"
)

type RDB struct {
	rdb    *redis.Client
	ctxTTL int
}

func NewRDB() (*RDB, error) {
	const op = "rdb.NewRDB"

	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_AI_ADDR"),
		Password: os.Getenv("REDIS_AI_PASSWORD"),
		DB:       0,
	})

	pingTimeout := utils.GetEnvInt("REDIS_PING_TIMEOUT", 10)
	ctxTTL := utils.GetEnvInt("REDIS_CTX_TTL", 10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(pingTimeout)*time.Second)
	defer cancel()

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("%s: redis ping: %w", op, err)
	}

	r := &RDB{
		rdb:    rdb,
		ctxTTL: ctxTTL,
	}

	return r, nil
}

func (r *RDB) Close() error {
	return r.rdb.Close()
}
