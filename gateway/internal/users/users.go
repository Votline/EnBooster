// Package users user.go implement Service interface
// and make connect to users-service by gRPC
package users

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"enbstr/internal/cbreaker"
	"enbstr/internal/ui"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	tele "gopkg.in/telebot.v3"
)

// UsersService is a struct that implements Service interface
// and makes connect to users-service by gRPC
type UsersService struct {
	name        string
	adminUUID   int64
	ctxTimeout  time.Duration
	log         *zap.Logger
	cb          *gobreaker.CircuitBreaker[any]
	conn        *grpc.ClientConn
	client      pb.UsersServiceClient
	kafkaWriter *kafka.Writer
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

// NewUS creates new UsersService instance
func NewUS(ctxTimeout time.Duration, adminUUID int64, log *zap.Logger) (*UsersService, error) {
	const op = "users.NewUS"

	log.Info("Creating users service",
		zap.String("op", op))

	rpcCert, err := getTLSConfig(os.Getenv("TLS_SERVER_NAME"), "ssl/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("%s: get certs: %w", op, err)
	}

	conn, err := grpc.NewClient(
		os.Getenv("USERS_HOST")+":"+os.Getenv("USERS_PORT"),
		grpc.WithTransportCredentials(credentials.NewTLS(rpcCert)))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create client: %w", op, err)
	}

	kafkaCert, err := getTLSConfig(os.Getenv("KAFKA_SERVER_NAME"), "ssl/kafka.crt")
	if err != nil {
		return nil, fmt.Errorf("%s: get certs: %w", op, err)
	}

	writer := &kafka.Writer{
		Addr:     kafka.TCP(os.Getenv("KAFKA_ADDR")),
		Topic:    os.Getenv("KAFKA_TOPIC_GTW_US"),
		Balancer: &kafka.LeastBytes{},
		Async:    true,
		Transport: &kafka.Transport{
			TLS: kafkaCert,
		},
	}

	return &UsersService{
		name:        "users",
		adminUUID:   adminUUID,
		ctxTimeout:  ctxTimeout,
		log:         log,
		conn:        conn,
		cb:          cbreaker.NewCB("users", log),
		client:      pb.NewUsersServiceClient(conn),
		kafkaWriter: writer,
	}, nil
}

// HandleRoutes handle user messages which intended for user-service
func (us *UsersService) HandleRoutes(msg string, c tele.Context) error {
	const op = "users.RegisterRoutes"

	switch msg {
	case "/start":
		reqTrace := uuid.NewString()
		uuid := c.Message().Sender.ID
		if err := us.Register(uuid, reqTrace); err != nil {
			return err
		}
		var menu *tele.ReplyMarkup
		if uuid == us.adminUUID {
			menu = ui.ReplyMenu([]string{"Learning", "Profile", "Help"})
		} else {
			menu = ui.ReplyMenu([]string{"Learning", "Profile"})
		}
		return c.Send("Welcome to EnBooster!", menu)
	case "Profile":
		reqTrace := uuid.NewString()
		uuid := c.Message().Sender.ID
		ud, err := us.GetData(uuid, reqTrace)
		if err != nil {
			return err
		}

		return c.Send(fmt.Sprintf("Your data:\nUUID: %d\nLevel: %s\nTask position:%d\nBest theme: %s | %d\nWorst theme: %s | %d\nStreak: %d",
			ud.UUID, ud.Level, ud.TaskID,
			ud.BestTheme, ud.BestThemeCnt,
			ud.WorstTheme, ud.WorstThemeCnt, ud.Streak))
	}

	return nil
}

// Close closes the connection to the server
func (us *UsersService) Close() error {
	const op = "users.Close"

	errStr := ""
	if err := us.kafkaWriter.Close(); err != nil {
		errStr += err.Error()
	}
	if err := us.conn.Close(); err != nil {
		errStr += err.Error()
	}
	if errStr != "" {
		return fmt.Errorf("%s: %s", op, errStr)
	}

	return nil
}

// GetName returns the name of the service
func (us *UsersService) GetName() string {
	return us.name
}
