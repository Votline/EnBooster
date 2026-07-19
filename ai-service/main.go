// Package main provides ai service grpc methods.
package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"unsafe"

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

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

var audioPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 4096)
		return &b
	},
}

const batchSizeThreehold = 128

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

func (s *aiserver) GenerateText(req *pb.GenerateTextReq, stream pb.AIService_GenerateTextServer) error {
	const op = "aiserver.GenerateText"

	uuid := req.GetUuid()
	prompt := req.GetPrompt()
	sysprompt := req.GetSystemPrompt()
	reqTrace := req.GetRequestTrace()

	s.log.Debug("Stream Generate text request received",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("prompt_length", len(prompt)),
		zap.Int("system_prompt_length", len(sysprompt)),
		zap.String("request_trace", reqTrace))

	uctx, err := s.rdb.GetUserContext(uuid)
	if err != nil {
		return fmt.Errorf("%s: : %w", op, err)
	}

	if uctx == nil {
		uctx = make([]int, 0)
	}

	s.log.Debug("User context received",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("user_context_length", len(uctx)))

	resBuf := bufPool.Get().(*bytes.Buffer)
	resBuf.Reset()
	defer bufPool.Put(resBuf)

	newUctx, err := s.rt.GenerateText(prompt, sysprompt, uctx, func(text string) {
		resBuf.WriteString(text)

		if resBuf.Len() > batchSizeThreehold {
			if err := stream.Send(&pb.GenerateTextRes{Text: resBuf.String()}); err != nil {
				s.log.Error("failed to send response", zap.String("op", op), zap.Error(err))
			}
			resBuf.Reset()
		}
	})
	if err != nil {
		return fmt.Errorf("%s: : %w", op, err)
	}

	if resBuf.Len() > 0 {
		if err := stream.Send(&pb.GenerateTextRes{Text: resBuf.String()}); err != nil {
			s.log.Error("failed to send response", zap.String("op", op), zap.Error(err))
		}
	}

	s.log.Debug("Generate response sent",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("res_length", resBuf.Len()),
		zap.String("request_trace", reqTrace))

	if err := s.rdb.SetUserContext(uuid, newUctx); err != nil {
		return fmt.Errorf("%s: : %w", op, err)
	}

	s.log.Debug("User context updated",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("user_context_length", len(newUctx)))

	return nil
}

func (s *aiserver) GenerateAudio(ctx context.Context, req *pb.GenerateAudioReq) (*pb.GenerateAudioRes, error) {
	const op = "aiserver.GenerateAudio"

	text := req.GetText()
	reqTrace := req.GetRequestTrace()

	s.log.Debug("Generate audio request received",
		zap.String("op", op),
		zap.Int("text_len", len(text)),
		zap.String("request_trace", reqTrace))

	audioBuf := audioPool.Get().(*[]byte)
	defer audioPool.Put(audioBuf)

	if err := s.rt.GenerateAudio(text, audioBuf, ctx); err != nil {
		return nil, fmt.Errorf("%s: : %w", op, err)
	}

	s.log.Debug("Generate audio response sent",
		zap.String("op", op),
		zap.Int("audio_len", len(*audioBuf)),
		zap.String("request_trace", reqTrace))

	audioTxt := unsafe.String(unsafe.SliceData(*audioBuf), len(*audioBuf))

	return &pb.GenerateAudioRes{AudioData: audioTxt}, nil
}
