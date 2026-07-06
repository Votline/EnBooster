// Package users handler.go contains methods for
// call users service grpc methods
package users

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unsafe"

	"enbstr/internal/services"
	stm "enbstr/internal/statemanager"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// UserData is a struct that represents user data
type UserData struct {
	UUID      int64  `json:"uuid"`
	BestTask  int32  `json:"best_task"`
	WorstTask int32  `json:"worst_task"`
	Level     string `json:"level"`
	TaskID    int32  `json:"task_id"`
	Streak    int32  `json:"streak"`
}

// UserAnswer used to push user answer to kafka
type UserAnswer struct {
	UUID      int64  `json:"uuid"`
	Correct   bool   `json:"correct"`
	Counter   int    `json:"counter"`
	Theme     string `json:"theme"`
	RequestID string `json:"request_id"`
}

// Register registers a new user
func (us *UsersService) Register(uuid int64, reqTrace string) error {
	const op = "users.Register"

	us.log.Debug("Register user request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), us.ctxTimeout*time.Second)
	defer cancel()

	if _, err := services.CallRPC(us.cb, func() (*pb.RegRes, error) {
		return us.client.RegUser(ctx, &pb.RegReq{
			Uuid:         uuid,
			RequestTrace: reqTrace,
		})
	}); err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	us.log.Debug("Register user successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	return nil
}

// GetData returns user data
func (us *UsersService) GetData(uuid int64, reqTrace string) (UserData, error) {
	const op = "users.GetData"

	us.log.Debug("Get user data request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), us.ctxTimeout*time.Second)
	defer cancel()

	res, err := services.CallRPC(us.cb, func() (*pb.GetRes, error) {
		return us.client.GetUser(ctx, &pb.GetReq{
			Uuid:         uuid,
			RequestTrace: reqTrace,
		})
	})
	if err != nil {
		return UserData{}, fmt.Errorf("%s: rpc call: %w", op, err)
	}

	dataBytes := unsafe.Slice(unsafe.StringData(res.Data), len(res.Data))

	var userData UserData
	if err := json.Unmarshal(dataBytes, &userData); err != nil {
		return UserData{}, fmt.Errorf("%s: unmarshal: %w", op, err)
	}

	us.log.Debug("Get user data successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	return userData, nil
}

// UpdateData updates user data
func (us *UsersService) UpdateData(uuid int64, data UserData, reqTrace string) error {
	const op = "users.UpdateData"

	us.log.Debug("Update user data request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), us.ctxTimeout*time.Second)
	defer cancel()

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}
	dataStr := unsafe.String(unsafe.SliceData(dataBytes), len(dataBytes))

	if _, err := services.CallRPC(us.cb, func() (*pb.UpdRes, error) {
		return us.client.UpdUser(ctx, &pb.UpdReq{
			Data:         dataStr,
			RequestTrace: reqTrace,
		})
	}); err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	us.log.Debug("Update user data successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	return nil
}

// DelUser deletes a user
func (us *UsersService) DelUser(uuid int64, reqTrace string) error {
	const op = "users.DelUser"

	us.log.Debug("Delete user request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), us.ctxTimeout*time.Second)
	defer cancel()

	if _, err := services.CallRPC(us.cb, func() (*pb.DelRes, error) {
		return us.client.DelUser(ctx, &pb.DelReq{
			Uuid:         uuid,
			RequestTrace: reqTrace,
		})
	}); err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	us.log.Debug("Delete user successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	return nil
}

// UpdateByAnswer push message to kafka with 'correct' field
// to update user streak
func (us *UsersService) UpdateByAnswer(uuid int64, correct bool, counter int, theme, reqTrace string) {
	const op = "users.UpdateByAnswer"

	us.log.Debug("Update user by answer request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), us.ctxTimeout*time.Second)
	defer cancel()

	event := UserAnswer{
		UUID:      uuid,
		Correct:   correct,
		RequestID: reqTrace,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		us.log.Error("Failed to marshal event",
			zap.String("op", op),
			zap.String("reqTrace", reqTrace),
			zap.Error(err))
		return
	}

	key := strconv.FormatInt(uuid, 10)
	keyBytes := unsafe.Slice(unsafe.StringData(key), len(key))

	if err := us.kafkaWriter.WriteMessages(ctx,
		kafka.Message{
			Key:   keyBytes,
			Value: payload,
		}); err != nil {
		us.log.Error("Failed to write message to kafka",
			zap.String("op", op),
			zap.String("reqTrace", reqTrace),
			zap.Error(err))
	}

	us.log.Debug("Update user by answer successfully pushed to kafka",
		zap.String("op", op),
		zap.String("reqTrace", reqTrace))
}

// UpdateUserTaskCtx get user context, update it previous counter
// and set new state
func (us *UsersService) UpdateUserTaskCtx(uuid int64, state int8, theme, reqTrace, answer string, add int, sm *stm.StateManager) error {
	const op = "users.UpdateUserTaskCtx"

	us.log.Debug("Update user task ctx request",
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace),
		zap.String("op", op))

	var taskSes stm.TaskSession
	var jsonData []byte

	uctx, err := sm.GetUserCtx(uuid)
	if err != nil {
		return fmt.Errorf("%s: get user state: %w", op, err)
	}
	uctxData := unsafe.Slice(unsafe.StringData(uctx.JSONData), len(uctx.JSONData))
	if uctx.JSONData != "" {
		if err := json.Unmarshal(uctxData, &taskSes); err != nil {
			return fmt.Errorf("%s: unmarshal: %w", op, err)
		}
	}

	curSes := stm.TaskSession{
		CurrentTheme: theme,
		Counter:      taskSes.Counter + add,
		Answer:       answer,
	}

	jsonData, err = json.Marshal(curSes)
	if err != nil {
		return fmt.Errorf("%s: marshal json: %w", op, err)
	}

	if err := sm.SetUserCtx(uuid, stm.StateTaskLearning, jsonData); err != nil {
		return fmt.Errorf("%s set state: %w", op, err)
	}

	us.log.Debug("Update user task ctx successfully",
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace),
		zap.String("op", op))

	return nil
}
