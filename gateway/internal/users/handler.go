// Package users handler.go contains methods for
// call users service grpc methods
package users

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"go.uber.org/zap"
)

type UserData struct {
	UUID      int64  `json:"uuid"`
	BestTask  int32  `json:"best_task"`
	WorstTask int32  `json:"worst_task"`
	Streak    int64  `json:"streak"`
	Level     string `json:"level"`
	TaskID    int32  `json:"task_id"`
}

func (us *UsersService) Register(uuid int64, reqTrace string) error {
	const op = "users.Register"

	us.log.Debug("Register user request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := us.client.RegUser(ctx, &pb.RegReq{
		Uuid:         uuid,
		RequestTrace: reqTrace,
	}); err != nil {
		return fmt.Errorf("%s: register user: %w", op, err)
	}

	us.log.Debug("Register user successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	return nil
}

func (us *UsersService) GetData(uuid int64, reqTrace string) (UserData, error) {
	const op = "users.GetData"

	us.log.Debug("Get user data request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := us.client.GetUser(ctx, &pb.GetReq{
		Uuid:         uuid,
		RequestTrace: reqTrace,
	})
	if err != nil {
		return UserData{}, fmt.Errorf("%s: get user data: %w", op, err)
	}

	userData := UserData{
		UUID:      uuid,
		BestTask:  resp.BestTask,
		WorstTask: resp.WorstTask,
		Streak:    resp.Streak,
		Level:     resp.Level,
		TaskID:    resp.TaskId,
	}

	us.log.Debug("Get user data successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	return userData, nil
}

func (us *UsersService) DelUser(uuid int64, reqTrace string) error {
	const op = "users.DelUser"

	us.log.Debug("Delete user request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := us.client.DelUser(ctx, &pb.DelReq{
		Uuid:         uuid,
		RequestTrace: reqTrace,
	}); err != nil {
		return fmt.Errorf("%s: delete user: %w", op, err)
	}

	us.log.Debug("Delete user successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	return nil
}
