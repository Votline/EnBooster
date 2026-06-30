// Package rdb provides Redis database connection and
// methods for caching data.
package rdb

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
)

type RDB struct {
	rdb *redis.Client
}

func NewRDB() (*RDB, error) {
	const op = "rdb.NewRDB"

	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_LRN_ADDR"),
		Password: os.Getenv("REDIS_LRN_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("%s: redis ping: %w", op, err)
	}

	return &RDB{rdb: rdb}, nil
}

func (r *RDB) Close() error {
	return r.rdb.Close()
}

func (r *RDB) CacheTasks(ctx context.Context, key, value string) error {
	const op = "rdb.CacheTasks"

	if err := r.rdb.Set(ctx, key, value, 30*time.Minute).Err(); err != nil {
		return fmt.Errorf("%s: redis set: %w", op, err)
	}

	return nil
}

func (r *RDB) GetTasks(ctx context.Context, key string) (string, error) {
	const op = "rdb.GetTasks"

	tasks, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("%s: redis get: %w", op, err)
	}

	return tasks, nil
}
