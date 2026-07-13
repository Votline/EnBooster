// Package main provides ai service grpc methods.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"aisrv/internal/rdb"
	"aisrv/internal/router"

	pb "github.com/Votline/EnBooster/protos/generated-ai"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// aiserver is the ai service implementation.
type aiserver struct {
	log *zap.Logger
	rdb *rdb.RDB
	rt  *router.Router
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

	rt := router.NewRouter()

	s := aiserver{rdb: rdb, rt: rt, log: log}
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

func (s *aiserver) GenerateText(ctx context.Context, req *pb.GenerateTextReq) (*pb.GenerateTextRes, error) {
	const op = "aiserver.GenerateText"

	uuid := req.GetUuid()
	text := req.GetText()
	reqTrace := req.GetRequestTrace()

	s.log.Debug("Generate text request received",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("text_length", len(text)),
		zap.String("request_trace", reqTrace))

	uctx, err := s.rdb.GetUserContext(uuid)
	if err != nil {
		return nil, fmt.Errorf("%s: : %w", op, err)
	}

	if uctx == nil {
		uctx = make([]int, 0)
	}

	s.log.Debug("User context received",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("user_context_length", len(uctx)))

	res, newUctx, err := s.rt.Generate(text, uctx)
	if err != nil {
		return nil, fmt.Errorf("%s: : %w", op, err)
	}

	s.log.Debug("Generate response sent",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("res_length", len(res)),
		zap.String("request_trace", reqTrace))

	if err := s.rdb.SetUserContext(uuid, newUctx); err != nil {
		return nil, fmt.Errorf("%s: : %w", op, err)
	}

	s.log.Debug("User context updated",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("user_context_length", len(newUctx)))

	return &pb.GenerateTextRes{
		Text: res,
	}, nil
}
