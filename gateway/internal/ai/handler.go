// Package ai handler.go contains methods for
// call ai service grpc methods
package ai

import (
	"context"
	"fmt"
	"time"

	"enbstr/internal/services"

	pb "github.com/Votline/EnBooster/protos/generated-ai"
	"go.uber.org/zap"
)

// GenerateText call ai-service Generate method
func (ai *AIService) GenerateText(uuid int64, text, reqTrace string) (string, error) {
	const op = "ai.GenerateText"

	ai.log.Debug("Generate text request",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.String("reqTrace", reqTrace))

	ctx, cancel := context.WithTimeout(context.Background(), ai.ctxTimeout*time.Second)
	defer cancel()

	res, err := services.CallRPC(ai.cb, func() (*pb.GenerateTextRes, error) {
		return ai.client.GenerateText(ctx, &pb.GenerateTextReq{
			Uuid:         uuid,
			Text:         text,
			RequestTrace: reqTrace,
		})
	})
	if err != nil {
		return "", fmt.Errorf("%s: rpc call: %w", op, err)
	}

	ai.log.Debug("Generate text successfully",
		zap.String("op", op),
		zap.Int64("uuid", uuid),
		zap.Int("text length", len(text)),
		zap.String("reqTrace", reqTrace))

	return res.Text, nil
}
