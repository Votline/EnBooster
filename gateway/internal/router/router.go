// Package router handles user messages and calls handlers
package router

import (
	"enbstr/internal/ui"
	"enbstr/internal/users"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

func Setup(bot *tele.Bot, log *zap.Logger) {
	usrsrv, err := users.NewUS(log)
	if err != nil {
		log.Fatal("Failed to create users service", zap.Error(err))
	}
	usrsrv.RegisterRoutes(bot)

	menu := ui.ReplyMenu([]string{
		"Начать учёбу 📚", "Помощь 🤔️", "Профиль 👤",
	})

	inline := ui.InlineMenu([]ui.InlineBtn{
		{Text: "<", Data: "action_prev"},
		{Text: ">", Data: "action_next"},
		{Text: "exit", Data: "action_exit"},
	})

	bot.Handle("Начать учёбу 📚", func(c tele.Context) error {
		return c.Send("Start lerning message", menu, inline)
	})

	bot.Handle("Помощь 🤔️", func(c tele.Context) error {
		return c.Send("Help message")
	})

	bot.Handle("\faction_next", func(c tele.Context) error {
		return c.Send("action_next")
	})

	bot.Handle("\faction_prev", func(c tele.Context) error {
		return c.Send("action_prev")
	})

	bot.Handle("\faction_exit", func(c tele.Context) error {
		return c.Send("action_exit")
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		return c.Send("Unknown command", menu)
	})
}
