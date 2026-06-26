// Package main provides learn service grpc methods.
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
	"google.golang.org/grpc"

	"go.uber.org/zap"
)

// learnservice provides grpc learn service methods.
type learnservice struct {
	log *zap.Logger
	pb.UnimplementedLearnServiceServer
}

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	lis, err := net.Listen("tcp", ":"+os.Getenv("LEARN_PORT"))
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	s := learnservice{log: log}
	srv := grpc.NewServer()
	pb.RegisterLearnServiceServer(srv, &s)
	log.Info("server started")

	if err := srv.Serve(lis); err != nil {
		log.Fatal("failed to serve", zap.Error(err))
	}
}

func (s *learnservice) NewTask(ctx context.Context, req *pb.NewTaskReq) (*pb.NewTaskRes, error) {
	const op = "learnservice.NewTask"

	data := req.GetTaskData()
	if data == "" {
		return nil, fmt.Errorf("%s: empty data", op)
	}

	return nil, nil
}

func (s *learnservice) GetTask(ctx context.Context, req *pb.GetTaskReq) (*pb.GetTaskRes, error) {
	const op = "learnservice.GetTask"

	id := req.GetTaskUuid()
	if id == "" {
		return nil, fmt.Errorf("%s: empty id", op)
	}

	return nil, nil
}

func (s *learnservice) DelTask(ctx context.Context, req *pb.DelTaskReq) (*pb.DelTaskRes, error) {
	const op = "learnservice.DelTask"

	id := req.GetTaskUuid()
	if id == "" {
		return nil, fmt.Errorf("%s: empty id", op)
	}

	return nil, nil
}

func (s *learnservice) NewWord(ctx context.Context, req *pb.NewWordReq) (*pb.NewWordRes, error) {
	const op = "learnservice.NewWord"

	data := req.GetWord()
	if data == "" {
		return nil, fmt.Errorf("%s: empty data", op)
	}

	return nil, nil
}

func (s *learnservice) GetWord(ctx context.Context, req *pb.GetWordReq) (*pb.GetWordRes, error) {
	const op = "learnservice.GetWord"

	id := req.GetWordUuid()
	if id == "" {
		return nil, fmt.Errorf("%s: empty id", op)
	}

	return nil, nil
}

func (s *learnservice) DelWord(ctx context.Context, req *pb.DelWordReq) (*pb.DelWordRes, error) {
	const op = "learnservice.DelWord"

	id := req.GetWordUuid()
	if id == "" {
		return nil, fmt.Errorf("%s: empty id", op)
	}

	return nil, nil
}
