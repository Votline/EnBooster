// Package users handler.go contains methods for
// call users service grpc methods
package users

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	tele "gopkg.in/telebot.v3"
)

type UserData struct {
	UUID      string `json:"uuid"`
	BestTask  int32  `json:"best_task"`
	WorstTask int32  `json:"worst_task"`
	Streak    int64  `json:"streak"`
}

func (us *UsersService) Register(tctx tele.Context) error {
	const op = "users.Register"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uuid := tctx.Sender().ID
	uuidStr := strconv.Itoa(int(uuid))

	if _, err := us.client.RegUser(ctx, &pb.RegReq{
		Uuid: uuidStr,
	}); err != nil {
		return fmt.Errorf("%s: register user: %w", op, err)
	}

	return nil
}

func (us *UsersService) GetData(tctx tele.Context) (UserData, error) {
	const op = "users.GetData"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uuid := tctx.Sender().ID
	uuidStr := strconv.Itoa(int(uuid))

	resp, err := us.client.GetUser(ctx, &pb.GetReq{
		Uuid: uuidStr,
	})
	if err != nil {
		return UserData{}, fmt.Errorf("%s: get user data: %w", op, err)
	}

	userData := UserData{
		UUID:      uuidStr,
		BestTask:  resp.BestTask,
		WorstTask: resp.WorstTask,
		Streak:    resp.Streak,
	}

	return userData, nil
}

func (us *UsersService) Delete(tctx tele.Context) error {
	const op = "users.Delete"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uuid := tctx.Sender().ID
	uuidStr := strconv.Itoa(int(uuid))

	if _, err := us.client.DelUser(ctx, &pb.DelReq{
		Uuid: uuidStr,
	}); err != nil {
		return fmt.Errorf("%s: delete user: %w", op, err)
	}

	return nil
}
