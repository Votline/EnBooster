// Package main provides learn service grpc methods.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"unsafe"

	"learn/internal/db"
	"learn/internal/parser"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
	"google.golang.org/grpc"

	"go.uber.org/zap"
)

// learnservice provides grpc learn service methods.
type learnservice struct {
	db  *db.DB
	log *zap.Logger
	pb.UnimplementedLearnServiceServer
}

var tasksPool = sync.Pool{
	New: func() any {
		t := make([]parser.Task, 0, 100)
		return &t
	},
}

var wordsPool = sync.Pool{
	New: func() any {
		w := make([]parser.Word, 0, 100)
		return &w
	},
}

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	lis, err := net.Listen("tcp", ":"+os.Getenv("LEARN_PORT"))
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	db, err := db.NewDB(log)
	if err != nil {
		log.Fatal("failed to create db", zap.Error(err))
	}

	s := learnservice{log: log, db: db}
	srv := grpc.NewServer()
	pb.RegisterLearnServiceServer(srv, &s)
	log.Info("server started")

	if err := srv.Serve(lis); err != nil {
		log.Fatal("failed to serve", zap.Error(err))
	}
}

// NewTask inserts new tasks into database.
func (s *learnservice) NewTask(ctx context.Context, req *pb.NewTaskReq) (*pb.NewTaskRes, error) {
	const op = "learnservice.NewTask"

	data := req.GetJsonData()
	if data == "" {
		return nil, fmt.Errorf("%s: empty data", op)
	}

	tasksPtr := tasksPool.Get().(*[]parser.Task)
	*tasksPtr = (*tasksPtr)[:0]
	defer tasksPool.Put(tasksPtr)

	if err := parser.ParseTask(data, tasksPtr); err != nil {
		return nil, fmt.Errorf("%s: parse json data: %w", op, err)
	}

	rowsAffected, err := s.db.NewTaskBulk(ctx, *tasksPtr)
	if err != nil {
		return nil, fmt.Errorf("%s: insert tasks: %w", op, err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("%s: no rows affected", op)
	}

	return &pb.NewTaskRes{Inserted: rowsAffected}, nil
}

func (s *learnservice) GetTask(ctx context.Context, req *pb.GetTaskReq) (*pb.GetTaskRes, error) {
	const op = "learnservice.GetTask"

	level := req.GetLevel()
	if level == "" {
		return nil, fmt.Errorf("%s: empty level", op)
	}
	pos := req.GetPosition()

	tasksPtr := tasksPool.Get().(*[]parser.Task)
	*tasksPtr = (*tasksPtr)[:0]
	defer tasksPool.Put(tasksPtr)

	if err := s.db.GetTask(ctx, level, pos, tasksPtr); err != nil {
		return nil, fmt.Errorf("%s: get tasks: %w", op, err)
	}

	tasksBytes, err := json.Marshal(*tasksPtr)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal tasks: %w", op, err)
	}
	tasksStr := unsafe.String(unsafe.SliceData(tasksBytes), len(tasksBytes))

	return &pb.GetTaskRes{Data: tasksStr}, nil
}

func (s *learnservice) DelTask(ctx context.Context, req *pb.DelTaskReq) (*pb.DelTaskRes, error) {
	const op = "learnservice.DelTask"

	return nil, nil
}

func (s *learnservice) NewWord(ctx context.Context, req *pb.NewWordsReq) (*pb.NewWordsRes, error) {
	const op = "learnservice.NewWord"

	return nil, nil
}

func (s *learnservice) GetWord(ctx context.Context, req *pb.GetWordReq) (*pb.GetWordRes, error) {
	const op = "learnservice.GetWord"

	return nil, nil
}

func (s *learnservice) DelWord(ctx context.Context, req *pb.DelWordReq) (*pb.DelWordRes, error) {
	const op = "learnservice.DelWord"

	return nil, nil
}
