// Package router handles user messages and calls handlers
package router

import (
	"enbstr/internal/ui"

	tele "gopkg.in/telebot.v3"
)

func Setup(bot *tele.Bot) {
	menu := ui.ReplyMenu([]string{
		"Начать учёбу 📚", "Помощь 🤔️", "Профиль 👤",
	})

	inline := ui.InlineMenu([]ui.InlineBtn{
		{Text: "<", Data: "action_prev"},
		{Text: ">", Data: "action_next"},
		{Text: "exit", Data: "action_exit"},
	})

	bot.Handle("/start", func(c tele.Context) error {
		return c.Send("Hello, I'm EnBooster!", menu)
	})

	bot.Handle("Начать учёбу 📚", func(c tele.Context) error {
		return c.Send("Start lerning message", menu, inline)
	})

	bot.Handle("Помощь 🤔️", func(c tele.Context) error {
		return c.Send("Help message")
	})

	bot.Handle("Профиль 👤", func(c tele.Context) error {
		return c.Send("Profile message")
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
