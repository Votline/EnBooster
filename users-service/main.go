// Package main provides users service grpc methods.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"users/internal/db"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// usersserver provides users service grpc methods.
type usersserver struct {
	db     *db.DB
	log    *zap.Logger
	reader *kafka.Reader
	pb.UnimplementedUsersServiceServer
}

// UserAnswer used to apply user answer from kafka
type UserAnswer struct {
	UUID      int64  `json:"uuid"`
	Correct   bool   `json:"correct"`
	RequestID string `json:"request_id"`
	Streak    int64
}

// getTLSConfig returns tls config from path with servername
func getTLSConfig(srvName, path string) (*tls.Config, error) {
	caCert, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("get certs: %w", err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	config := &tls.Config{
		RootCAs:    certPool,
		ServerName: srvName,
	}

	return config, nil
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

	pdb, err := db.NewDB(log)
	if err != nil {
		log.Fatal("failed to create db", zap.Error(err))
	}
	defer pdb.Close()

	kafkaTLS, err := getTLSConfig(os.Getenv("KAFKA_SERVER_NAME"), "ssl/kafka.crt")
	if err != nil {
		log.Fatal("failed to get TLS config", zap.Error(err))
	}

	ctxTimeout := time.Duration(db.GetEnvInt("CTX_TIMEOUT", 10))
	dialer := &kafka.Dialer{
		Timeout:   ctxTimeout * time.Second,
		DualStack: true,
		TLS:       kafkaTLS,
	}

	sesTimeout := time.Duration(db.GetEnvInt("KAFKA_SESSION_TIMEOUT", 6))
	heartbeatIntevarl := time.Duration(db.GetEnvInt("KAFKA_HEARTB_INTERVAL", 2))
	maxWait := time.Duration(db.GetEnvInt("KAFKA_MAX_WAIT", 100))

	kafkaAddr := os.Getenv("KAFKA_ADDR")
	kafkaTopic := os.Getenv("KAFKA_TOPIC_GTW_US")

	ensureTopics(kafkaAddr, kafkaTopic, kafkaTLS)

	log.Debug("Kafka topics successfully created")

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaAddr},
		Topic:    kafkaTopic,
		GroupID:  os.Getenv("KAFKA_GROUP_ID"),
		MinBytes: db.GetEnvInt("KAFKA_MIN_BYTES", 1),
		MaxBytes: db.GetEnvInt("KAFKA_MAX_BYTES", 10e6),
		Dialer:   dialer,

		StartOffset:       kafka.FirstOffset,
		SessionTimeout:    sesTimeout * time.Second,
		HeartbeatInterval: heartbeatIntevarl * time.Second,
		MaxWait:           maxWait * time.Millisecond,

		WatchPartitionChanges: true,
	})
	defer reader.Close()

	log.Debug("Kafka reader successfully created")

	s := usersserver{log: log, db: pdb, reader: reader}
	srv := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterUsersServiceServer(srv, &s)

	log.Debug("Users service successfully started")

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Fatal("failed to serve: ", zap.Error(err))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.ApplyAnswer(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	cancel()
	gracefulShutdown(&s, srv)
}

func ensureTopics(kafkaAddr, kafkaTopic string, kafkaTLS *tls.Config) error {
	const op = "usersserver.ensureTopics"
	client := &kafka.Client{
		Addr:    kafka.TCP(kafkaAddr),
		Timeout: 5 * time.Second,
		Transport: &kafka.Transport{
			TLS: kafkaTLS,
		},
	}

	resp, err := client.CreateTopics(context.Background(), &kafka.CreateTopicsRequest{
		Topics: []kafka.TopicConfig{
			{
				Topic:             kafkaTopic,
				NumPartitions:     1,
				ReplicationFactor: 1,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("%s: kafka create topics: %w", op, err)
	}

	for _, topicErr := range resp.Errors {
		if topicErr != nil {
			if errors.Is(topicErr, kafka.TopicAlreadyExists) {
				return nil
			}
			return fmt.Errorf("%s: kafka failed to create topic: %w", op, topicErr)
		}
	}

	return nil
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

	ud.UUID = uuid

	jsonData, err := json.Marshal(ud)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal user: %w", op, err)
	}
	jsonStr := unsafe.String(unsafe.SliceData(jsonData), len(jsonData))

	return &pb.GetRes{
		Data: jsonStr,
	}, nil
}

func (s *usersserver) UpdUser(ctx context.Context, req *pb.UpdReq) (*pb.UpdRes, error) {
	const op = "usersserver.UpdUser"

	data := req.GetData()
	if data == "" {
		return nil, fmt.Errorf("%s: empty data", op)
	}
	dataBytes := unsafe.Slice(unsafe.StringData(data), len(data))

	reqTrace := req.GetRequestTrace()
	s.log.Debug("UpdUser request",
		zap.Int("data len", len(data)),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	var user db.User
	if err := json.Unmarshal(dataBytes, &user); err != nil {
		return nil, fmt.Errorf("%s: unmarshal data: %w", op, err)
	}

	if err := s.db.UpdUser(user, ctx, reqTrace); err != nil {
		return nil, fmt.Errorf("%s: db update user: %w", op, err)
	}

	s.log.Debug("Successfully updated user",
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return &pb.UpdRes{}, nil
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

// ApplyAnswer used to apply user answer from kafka
// and update user streak
func (s *usersserver) ApplyAnswer(ctx context.Context) {
	const op = "usersserver.ApplyAnswer"

	ctxTimeout := time.Duration(db.GetEnvInt("CTX_TIMEOUT", 10))

	for {
		select {
		case <-ctx.Done():
			s.log.Debug("Kafka consumer stopped gracefully",
				zap.String("op", op))
			return
		default:
			loopCtx, cancel := context.WithTimeout(ctx, ctxTimeout*time.Second)

			msg, err := s.reader.FetchMessage(loopCtx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					cancel()
					time.Sleep(time.Second)
					continue
				}
				if strings.Contains(err.Error(), "Group Coordinator Not Available") {
					cancel()
					time.Sleep(time.Second)
					continue
				}
				if errors.Is(err, context.DeadlineExceeded) {
					cancel()
					continue
				}
				s.log.Error("Failed to read kafka message",
					zap.Error(err),
					zap.String("op", op))
				cancel()
				continue
			}

			var event UserAnswer
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				s.log.Error("Failed to unmarshal kafka message",
					zap.Error(err),
					zap.String("op", op))
				s.reader.CommitMessages(loopCtx, msg)
				cancel()
				continue
			}

			event.Streak = time.Now().Unix()

			if err := s.db.UpdateStreak(event.UUID, loopCtx, event.RequestID, event.Correct); err != nil {
				s.log.Error("Failed to update streak",
					zap.Error(err),
					zap.String("op", op))
				cancel()
				continue
			}

			if err := s.reader.CommitMessages(loopCtx, msg); err != nil {
				s.log.Error("Failed to commit kafka message",
					zap.Error(err),
					zap.String("op", op))
				cancel()
				continue
			}

			s.log.Debug("Successfully updated streak",
				zap.Int64("uuid", event.UUID),
				zap.String("request_id", event.RequestID),
				zap.String("op", op))

			cancel()
		}
	}
}
