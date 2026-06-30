// Package statemanager contains states for each user
// and helps methods to work with states
package statemanager

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	StateNone = iota
	StateWaitGetTask
)

type StateManager struct {
	rdb *redis.Client
}

// NewSM connects to redis and returns a new StateManager
func NewSM() (*StateManager, error) {
	const op = "statemanager.NewSM"

	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_SM_ADDR"),
		Password: os.Getenv("REDIS_SM_PASSWORD"),
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("%s: ping: %w", op, err)
	}

	return &StateManager{
		rdb: rdb,
	}, nil
}

// Close closes the connection to redis
func (sm *StateManager) Close() error {
	return sm.rdb.Close()
}

// GetState returns the state of the user from redis
func (sm *StateManager) GetState(uuid int64) (int8, error) {
	const op = "statemanager.GetState"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := "users:state" + strconv.FormatInt(uuid, 10)

	val, err := sm.rdb.Get(ctx, key).Result()
	if err != nil {
		return StateNone, fmt.Errorf("%s: failed to get state: %w", op, err)
	}

	state, err := strconv.ParseInt(val, 10, 8)
	if err != nil {
		return StateNone, fmt.Errorf("%s: failed to parse state: %w", op, err)
	}

	return int8(state), nil
}

// SetState sets the state of the user in redis
func (sm *StateManager) SetState(uuid int64, state int8) error {
	const op = "statemanager.SetState"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := "users:state" + strconv.FormatInt(uuid, 10)

	if _, err := sm.rdb.Set(ctx, key, state, 30*time.Minute).Result(); err != nil {
		return fmt.Errorf("%s: failed to set state: %w", op, err)
	}

	return nil
}
