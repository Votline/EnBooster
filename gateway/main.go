// Package main creates a Telegram bot and zap logger.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"enbstr/internal/router"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

func main() {
	log, err := createLogger()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	b, err := createBot()
	if err != nil {
		log.Fatal("Failed to create bot", zap.Error(err))
	}

	srv := router.Setup(b, log)

	log.Info("Bot is successfully created")

	go func() {
		srv.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	gracefulShutdown(srv, log)
}

func gracefulShutdown(srv *router.Server, log *zap.Logger) {
	const op = "gateway.gracefulShutdown"

	log.Info("Shutting down server", zap.String("op", op))

	srv.Close()

	log.Info("Server shutdown successfully", zap.String("op", op))
}

// createLogger creates a production zap logger.
func createLogger() (*zap.Logger, error) {
	const op = "main.createLogger"

	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create logger: %w", op, err)
	}

	return logger, nil
}

// createBot creates a Telegram bot.
// It uses the BOT_TOKEN environment variable.
// Timeout is set to 10 seconds.
func createBot() (*tele.Bot, error) {
	const op = "main.createBot"

	pollerTimeout := router.GetEnvInt("POLLER_TIMEOUT", 10)

	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: time.Duration(pollerTimeout) * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create bot: %w", op, err)
	}

	return b, nil
}
