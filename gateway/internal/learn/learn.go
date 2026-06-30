// Package learn learn.go implement Service interface
// and make connect to learn-service by gRPC
package learn

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"enbstr/internal/statemanager"

	pb "github.com/Votline/EnBooster/protos/generated-learn"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	tele "gopkg.in/telebot.v3"
)

type LearnService struct {
	name   string
	log    *zap.Logger
	conn   *grpc.ClientConn
	client pb.LearnServiceClient
	states *statemanager.StateManager
}

func NewLS(states *statemanager.StateManager, log *zap.Logger) (*LearnService, error) {
	const op = "learn.NewLS"

	log.Info("Creating learn service",
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
		os.Getenv("LEARN_HOST")+":"+os.Getenv("LEARN_PORT"),
		grpc.WithTransportCredentials(credentials.NewTLS(config)))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create client: %w", op, err)
	}

	return &LearnService{
		name:   "learn",
		log:    log,
		conn:   conn,
		client: pb.NewLearnServiceClient(conn),
		states: states,
	}, nil
}

func (ls *LearnService) RegisterRoutes(bot *tele.Bot) error {
	const op = "learn.RegisterRoutes"

	return nil
}

func (ls *LearnService) Close() error {
	return ls.conn.Close()
}

func (ls *LearnService) GetName() string {
	return ls.name
}
