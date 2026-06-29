// Package router handles user messages and calls handlers
package router

import (
	"fmt"
	"sync"

	"enbstr/internal/learn"
	"enbstr/internal/statemanager"
	"enbstr/internal/users"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

var tasksList = sync.Pool{
	New: func() any {
		l := make([]learn.Task, 0)
		return &l
	},
}

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

	bot.Handle("Начать учёбу 📚", func(c tele.Context) error {
		reqTrace := uuid.NewString()

		data, err := usrsrv.GetData(c.Sender().ID, reqTrace)
		if err != nil {
			return fmt.Errorf("get user data: %w", err)
		}

		tasksPtr := tasksList.Get().(*[]learn.Task)
		defer tasksList.Put(tasksPtr)

		if err := lrnsrv.GetTasks(data.Level, data.TaskID, tasksPtr, reqTrace); err != nil {
			return fmt.Errorf("get tasks: %w", err)
		}

		return c.Send(fmt.Sprintf("Список заданий:\n%v", *tasksPtr))
	})
}
