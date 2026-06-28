// Package learn tasks.go calls learn-service grpc methods
package learn

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"
	"unsafe"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
	tele "gopkg.in/telebot.v3"
)

func (ls *LearnService) NewTasks(tctx tele.Context) error {
	const op = "learn.NewTasks"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	jsonData := tctx.Message().Text

	res, err := ls.client.NewTask(ctx, &pb.NewTaskReq{
		JsonData: jsonData,
	})
	if err != nil {
		return fmt.Errorf("%s: new task: %w", op, err)
	}

	return tctx.Send(fmt.Sprintf("Insterted: %d", res.Inserted))
}

func (ls *LearnService) GetTasks(tctx tele.Context) error {
	const op = "learn.GetTasks"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userState := ls.states.GetState(tctx.Chat().ID)
	level := userState.Level
	pos := userState.Pos

	tasks, err := ls.client.GetTask(ctx, &pb.GetTaskReq{
		Level:    level,
		Position: pos,
	})
	if err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	return tctx.Send(fmt.Sprintf("Level: %s\nPosition: %d\nTasks: %v", level, pos, tasks))
}

func (ls *LearnService) DeleteTasks(tctx tele.Context) error {
	const op = "learn.DeleteTasks"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg := tctx.Message().Text
	msgBytes := unsafe.Slice(unsafe.StringData(msg), len(msg))
	level, pos, has := bytes.Cut(msgBytes, []byte(" "))
	if !has {
		return fmt.Errorf("%s: cut message: invalid message structure", op)
	}

	levelStr := unsafe.String(unsafe.SliceData(level), len(level))
	posStr := unsafe.String(unsafe.SliceData(pos), len(pos))
	posInt, err := strconv.Atoi(posStr)
	if err != nil {
		return fmt.Errorf("%s: atoi position: invalid position", op)
	}

	if _, err := ls.client.DelTask(ctx, &pb.DelTaskReq{
		Level:    levelStr,
		Position: int32(posInt),
	}); err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	return tctx.Send(fmt.Sprintf("Level: %s\nPosition: %d", level, posInt))
}
