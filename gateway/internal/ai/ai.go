// Package ai ai.go implement Service interface
// and make connect to ai-service by gRPC
package ai

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"enbstr/internal/cbreaker"
	"enbstr/internal/ui"

	pb "github.com/Votline/EnBooster/protos/generated-ai"
	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	tele "gopkg.in/telebot.v3"
)

// AIService is a struct that implements Service interface
// and makes connect to ai-service by gRPC
type AIService struct {
	name       string
	uiInstns   *ui.UI
	ctxTimeout time.Duration
	log        *zap.Logger
	cb         *gobreaker.CircuitBreaker[any]
	conn       *grpc.ClientConn
	client     pb.AIServiceClient
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

// NewAIS creates new AIService instance
func NewAIS(ctxTimeout time.Duration, uiInstns *ui.UI, log *zap.Logger) (*AIService, error) {
	const op = "ai.NewAIS"

	log.Info("Creating ai service",
		zap.String("op", op))

	rpcCert, err := getTLSConfig(os.Getenv("TLS_SERVER_NAME"), "ssl/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("%s: get certs: %w", op, err)
	}

	conn, err := grpc.NewClient(
		os.Getenv("AISRV_HOST")+":"+os.Getenv("AISRV_PORT"),
		grpc.WithTransportCredentials(credentials.NewTLS(rpcCert)))
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create client: %w", op, err)
	}

	return &AIService{
		name:       "ai",
		uiInstns:   uiInstns,
		ctxTimeout: ctxTimeout,
		log:        log,
		conn:       conn,
		cb:         cbreaker.NewCB("ai", log),
		client:     pb.NewAIServiceClient(conn),
	}, nil
}

// HandleRoutes handle user messages which intended for ai-service
func (ai *AIService) HandleRoutes(msg string, c tele.Context) error {
	const op = "ai.RegisterRoutes"
	return nil
}

// Close closes the connection to the server
func (ai *AIService) Close() error {
	return ai.conn.Close()
}

// GetName returns the name of the service
func (ai *AIService) GetName() string {
	return ai.name
}
