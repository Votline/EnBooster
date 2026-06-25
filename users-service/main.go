// Package main provides users service grpc methods.
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"users/internal/db"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// usersserver provides users service grpc methods.
type usersserver struct {
	db  *db.DB
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

	db, err := db.NewDB(log)
	if err != nil {
		log.Fatal("failed to create db: ", zap.Error(err))
	}
	defer db.Close()

	s := usersserver{log: log, db: db}
	srv := grpc.NewServer()
	pb.RegisterUsersServiceServer(srv, &s)

	log.Info("Users service succesfully started")

	if err := srv.Serve(lis); err != nil {
		log.Fatal("failed to serve: ", zap.Error(err))
	}
}

// RegUser add user to database with uuid from request.
func (s *usersserver) RegUser(ctx context.Context, req *pb.RegReq) (*pb.RegRes, error) {
	const op = "usersserver.RegUser"
	uuid := req.GetUuid()
	if uuid == "" {
		return nil, fmt.Errorf("%s: empty uuid", op)
	}

	s.log.Info("RegUser", zap.String("uuid", uuid))

	if err := s.db.RegUser(uuid, ctx); err != nil {
		return nil, fmt.Errorf("%s: db insert user: %w", op, err)
	}

	s.log.Info("Successfully registered user", zap.String("uuid", uuid))

	return nil, nil
}

// GetUser get user from database with uuid from request.
// Returns all user fields if user exists
func (s *usersserver) GetUser(ctx context.Context, req *pb.GetReq) (*pb.GetRes, error) {
	const op = "usersserver.GetUser"
	uuid := req.GetUuid()
	if uuid == "" {
		s.log.Error("GetUser", zap.Error(fmt.Errorf("%s: empty uuid", op)))
		return nil, fmt.Errorf("%s: empty uuid", op)
	}

	s.log.Info("GetUser", zap.String("uuid", uuid))

	ud, err := s.db.GetUser(uuid, ctx)
	if err != nil {
		s.log.Error("GetUser", zap.Error(err))
		return nil, fmt.Errorf("%s: db get user: %w", op, err)
	}

	s.log.Info("Successfully got user",
		zap.Int("best task", int(ud.BestTask)),
		zap.Int("worst task", int(ud.WorstTask)),
		zap.Int("steak", int(ud.Streak)))

	return &pb.GetRes{
		BestTask:  ud.BestTask,
		WorstTask: ud.WorstTask,
		Streak:    ud.Streak,
	}, nil
}

// DelUser delete user from database with uuid from request.
func (s *usersserver) DelUser(ctx context.Context, req *pb.DelReq) (*pb.DelRes, error) {
	const op = "usersserver.DelUser"
	uuid := req.GetUuid()
	if uuid == "" {
		return nil, fmt.Errorf("%s: empty uuid", op)
	}

	s.log.Info("DelUser", zap.String("uuid", uuid))

	if err := s.db.DelUser(uuid, ctx); err != nil {
		return nil, fmt.Errorf("%s: db delete user: %w", op, err)
	}

	s.log.Info("Successfully deleted user", zap.String("uuid", uuid))

	return nil, nil
}
