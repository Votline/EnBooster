// Package main provides ai service grpc methods.
package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"aisrv/internal/rdb"

	pb "github.com/Votline/EnBooster/protos/generated-ai"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// aiserver is the ai service implementation.
type aiserver struct {
	log *zap.Logger
	rdb *rdb.RDB
	pb.UnimplementedAIServiceServer
}

func main() {
	log, _ := zap.NewDevelopment()
	defer log.Sync()

	creds, err := credentials.NewServerTLSFromFile("ssl/server.crt", "ssl/server.key")
	if err != nil {
		log.Fatal("failed to create credentials", zap.Error(err))
	}

	lis, err := net.Listen("tcp", ":"+os.Getenv("AISRV_PORT"))
	if err != nil {
		log.Fatal("failed to listen", zap.Error(err))
	}

	rdb, err := rdb.NewRDB()
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	s := aiserver{rdb: rdb, log: log}
	srv := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterAIServiceServer(srv, &s)

	log.Debug("AI service successfully started")

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatal("failed to serve", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	gracefulShutdown(&s, srv)
}

func gracefulShutdown(s *aiserver, srv *grpc.Server) {
	const op = "aiserver.gracefulShutdown"

	s.log.Info("Shutting down server", zap.String("op", op))

	srv.Stop()
	s.log.Info("Server stopped", zap.String("op", op))

	s.log.Info("Server shutdown successfully", zap.String("op", op))
}
