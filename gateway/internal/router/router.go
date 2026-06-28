// Package router handles user messages and calls handlers
package router

import (
	"enbstr/internal/learn"
	"enbstr/internal/statemanager"
	"enbstr/internal/users"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

func Setup(bot *tele.Bot, log *zap.Logger) {
	states, err := statemanager.NewSM()
	if err != nil {
		log.Fatal("Failed to create state manager", zap.Error(err))
	}
	log.Info("Successfully connected redis")

	setupServices(bot, states, log)
}

func setupServices(bot *tele.Bot, states *statemanager.StateManager, log *zap.Logger) {
	usrsrv, err := users.NewUS(log)
	if err != nil {
		log.Fatal("Failed to create users service", zap.Error(err))
	}
	usrsrv.RegisterRoutes(bot)

	lrnsrv, err := learn.NewLS(states, log)
	if err != nil {
		log.Fatal("Failed to create learn service", zap.Error(err))
	}
	lrnsrv.RegisterRoutes(bot)
}
