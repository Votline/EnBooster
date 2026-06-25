package main

import (
	"fmt"
	"os"
	"time"

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

	b.Handle("/ping", func(c tele.Context) error {
		return c.Send("pong")
	})

	log.Info("Bot is successfully created")

	b.Start()
}

func createLogger() (*zap.Logger, error) {
	const op = "main.createLogger"

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create logger: %w", op, err)
	}

	return logger, nil
}

func createBot() (*tele.Bot, error) {
	const op = "main.createBot"

	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create bot: %w", op, err)
	}

	return b, nil
}
