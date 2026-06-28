// Package learn tasks.go calls learn-service grpc methods
package learn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unsafe"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
)

type Task struct {
	TaskData string `json:"task_data"`
	Level    string `json:"level"`
	Answer   string `json:"answer"`
	Position int32  `json:"position"`
}

func (ls *LearnService) NewTasks(msg string) (int32, error) {
	const op = "learn.NewTasks"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := ls.client.NewTasks(ctx, &pb.NewTasksReq{
		JsonData: msg,
	})
	if err != nil {
		return -1, fmt.Errorf("%s: new task: %w", op, err)
	}

	return res.Inserted, nil
}

func (ls *LearnService) GetTasks(level string, pos int32, tasksList *[]Task) error {
	const op = "learn.GetTasks"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasks, err := ls.client.GetTasks(ctx, &pb.GetTasksReq{
		Level:    level,
		Position: pos,
	})
	if err != nil {
		return fmt.Errorf("%s: rpc call: %w", op, err)
	}

	tasksData := tasks.Data
	tasksBytes := unsafe.Slice(unsafe.StringData(tasksData), len(tasksData))

	if err := json.Unmarshal(tasksBytes, tasksList); err != nil {
		return fmt.Errorf("%s: unmarshal tasks: %w", op, err)
	}

	return nil
}

func (ls *LearnService) DeleteTasks(msg string) error {
	const op = "learn.DeleteTasks"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	return nil
}
