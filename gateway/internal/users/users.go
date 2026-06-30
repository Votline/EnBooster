// Package users user.go implement Service interface
// and make connect to users-service by gRPC
package users

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"enbstr/internal/cbreaker"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"github.com/google/uuid"
	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	tele "gopkg.in/telebot.v3"
)

type UsersService struct {
	name   string
	log    *zap.Logger
	cb     *gobreaker.CircuitBreaker[any]
	conn   *grpc.ClientConn
	client pb.UsersServiceClient
}

func NewUS(log *zap.Logger) (*UsersService, error) {
	const op = "users.NewUS"

	log.Info("Creating users service",
		zap.String("op", op))

	caCert, err := os.ReadFile("ssl/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("%s: get certs: %w", op, err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	config := &tls.Config{
		RootCAs:    certPool,
		ServerName: os.Getenv("TLS_SERVER_NAME"),
	}

	conn, err := grpc.NewClient(
		os.Getenv("USERS_HOST")+":"+os.Getenv("USERS_PORT"),
		grpc.WithTransportCredentials(credentials.NewTLS(config)))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create client: %w", op, err)
	}

	return &UsersService{
		name:   "users",
		log:    log,
		conn:   conn,
		cb:     cbreaker.NewCB("users", log),
		client: pb.NewUsersServiceClient(conn),
	}, nil
}

func (us *UsersService) RegisterRoutes(bot *tele.Bot) error {
	const op = "users.RegisterRoutes"

	bot.Handle("/start", func(c tele.Context) error {
		reqTrace := uuid.NewString()
		uuid := c.Message().Sender.ID
		if err := us.Register(uuid, reqTrace); err != nil {
			return err
		}
		return c.Send("Welcome to EnBooster!")
	})

	bot.Handle("Профиль 👤", func(c tele.Context) error {
		reqTrace := uuid.NewString()
		uuid := c.Message().Sender.ID
		ud, err := us.GetData(uuid, reqTrace)
		if err != nil {
			return err
		}

		return c.Send(fmt.Sprintf("Your data:\nUUID: %d\nBest task: %d\nWorst task: %d\nStreak: %d",
			ud.UUID, ud.BestTask, ud.WorstTask, ud.Streak))
	})

	return nil
}

func (us *UsersService) Close() error {
	return us.conn.Close()
}

func (us *UsersService) GetName() string {
	return us.name
}
