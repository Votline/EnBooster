package main

import (
	"context"
	"fmt"
	"net"
	"os"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type usersserver struct {
	log *zap.Logger
	pb.UnimplementedUsersServiceServer
}

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	lis, err := net.Listen("tcp", ":"+os.Getenv("USERS_PORT"))
	if err != nil {
		log.Fatal("failed to listen: ", zap.Error(err))
	}

	s := usersserver{log: log}
	srv := grpc.NewServer()
	pb.RegisterUsersServiceServer(srv, &s)

	log.Info("Users service succesfully started")

	if err := srv.Serve(lis); err != nil {
		log.Fatal("failed to serve: ", zap.Error(err))
	}
}

func (s *usersserver) RegUser(ctx context.Context, req *pb.RegReq) (*pb.RegRes, error) {
	const op = "usersserver.RegUser"
	uuid := req.GetUuid()
	if uuid == "" {
		return nil, fmt.Errorf("%s: empty uuid", op)
	}

	return nil, nil
}

func (s *usersserver) GetUser(ctx context.Context, req *pb.GetReq) (*pb.GetRes, error) {
	const op = "usersserver.GetUser"
	uuid := req.GetUuid()
	if uuid == "" {
		return nil, fmt.Errorf("%s: empty uuid", op)
	}

	return nil, nil
}

func (s *usersserver) DelUser(ctx context.Context, req *pb.DelReq) (*pb.DelRes, error) {
	const op = "usersserver.DelUser"
	uuid := req.GetUuid()
	if uuid == "" {
		return nil, fmt.Errorf("%s: empty uuid", op)
	}

	return nil, nil
}
