// Package users user.go implement Service interface
// and make connect to users-service by gRPC
package users

import (
	"fmt"
	"os"

	"enbstr/internal/services"

	pb "github.com/Votline/EnBooster/protos/generated-users"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	tele "gopkg.in/telebot.v3"
)

type UsersService struct {
	name   string
	log    *zap.Logger
	conn   *grpc.ClientConn
	client pb.UsersServiceClient
}

func NewUS(log *zap.Logger) (services.Service, error) {
	const op = "users.NewUS"

	log.Info("Creating users service",
		zap.String("op", op))

	conn, err := grpc.NewClient(
		os.Getenv("USERS_HOST")+":"+os.Getenv("USERS_PORT"),
		grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create client: %w", op, err)
	}

	return &UsersService{
		name:   "users",
		log:    log,
		conn:   conn,
		client: pb.NewUsersServiceClient(conn),
	}, nil
}

func (us *UsersService) RegisterRoutes(bot *tele.Bot) error {
	const op = "users.RegisterRoutes"

	bot.Handle("/start", func(c tele.Context) error {
		if err := us.Register(c); err != nil {
			return err
		}
		return c.Send("Welcome to EnBooster!")
	})

	bot.Handle("Профиль 👤", func(c tele.Context) error {
		ud, err := us.GetData(c)
		if err != nil {
			return err
		}

		return c.Send(fmt.Sprintf("Your data:\nUUID: %s\nBest task: %d\nWorst task: %d\nStreak: %d",
			ud.UUID, ud.BestTask, ud.WorstTask, ud.Streak))
	})

	return nil
}

func (us *UsersService) Close() error {
	return us.conn.Close()
}
