// Package main provides users service grpc methods.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"users/internal/db"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// usersserver provides users service grpc methods.
type usersserver struct {
	db  *db.DB
	log *zap.Logger
	pb.UnimplementedUsersServiceServer
}

func main() {
	log, _ := zap.NewDevelopment()
	defer log.Sync()

	creds, err := credentials.NewServerTLSFromFile("ssl/server.crt", "ssl/server.key")
	if err != nil {
		log.Fatal("failed to load TLS keys", zap.Error(err))
	}

	lis, err := net.Listen("tcp", ":"+os.Getenv("USERS_PORT"))
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	db, err := db.NewDB(log)
	if err != nil {
		log.Fatal("failed to create db", zap.Error(err))
	}
	defer db.Close()

	s := usersserver{log: log, db: db}
	srv := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterUsersServiceServer(srv, &s)

	log.Debug("Users service successfully started")

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatal("failed to serve: ", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	gracefulShutdown(&s, srv)
}

func gracefulShutdown(s *usersserver, srv *grpc.Server) {
	const op = "usersserver.gracefulShutdown"

	s.log.Info("Shutting down server", zap.String("op", op))

	srv.Stop()
	s.log.Info("Server stopped", zap.String("op", op))

	if err := s.db.Close(); err != nil {
		s.log.Error("Failed to close database",
			zap.String("op", op),
			zap.Error(err))
	}
	s.log.Info("Database closed", zap.String("op", op))

	s.log.Info("Server shutdown successfully", zap.String("op", op))
}

// RegUser add user to database with uuid from request.
func (s *usersserver) RegUser(ctx context.Context, req *pb.RegReq) (*pb.RegRes, error) {
	const op = "usersserver.RegUser"
	uuid := req.GetUuid()
	if uuid == 0 {
		return nil, fmt.Errorf("%s: empty uuid", op)
	}
	reqTrace := req.GetRequestTrace()

	s.log.Debug("RegUser request",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	if err := s.db.RegUser(uuid, ctx, reqTrace); err != nil {
		return nil, fmt.Errorf("%s: db insert user: %w", op, err)
	}

	s.log.Debug("Successfully registered user",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return &pb.RegRes{}, nil
}

// GetUser get user from database with uuid from request.
// Returns all user fields if user exists
func (s *usersserver) GetUser(ctx context.Context, req *pb.GetReq) (*pb.GetRes, error) {
	const op = "usersserver.GetUser"
	uuid := req.GetUuid()
	if uuid == 0 {
		s.log.Error("GetUser", zap.Error(fmt.Errorf("%s: empty uuid", op)))
		return nil, fmt.Errorf("%s: empty uuid", op)
	}
	reqTrace := req.GetRequestTrace()

	s.log.Debug("GetUser request",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	ud, err := s.db.GetUser(uuid, ctx, reqTrace)
	if err != nil {
		return nil, fmt.Errorf("%s: db get user: %w", op, err)
	}

	s.log.Debug("Successfully got user",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return &pb.GetRes{
		BestTask:  ud.BestTask,
		WorstTask: ud.WorstTask,
		Streak:    ud.Streak,
		TaskId:    ud.TaskID,
		Level:     ud.Level,
	}, nil
}

// DelUser delete user from database with uuid from request.
func (s *usersserver) DelUser(ctx context.Context, req *pb.DelReq) (*pb.DelRes, error) {
	const op = "usersserver.DelUser"
	uuid := req.GetUuid()
	if uuid == 0 {
		return nil, fmt.Errorf("%s: empty uuid", op)
	}
	reqTrace := req.GetRequestTrace()

	s.log.Debug("DelUser request",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	if err := s.db.DelUser(uuid, ctx, reqTrace); err != nil {
		return nil, fmt.Errorf("%s: db delete user: %w", op, err)
	}

	s.log.Debug("Successfully deleted user",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return &pb.DelRes{}, nil
}
