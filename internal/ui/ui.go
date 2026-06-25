// Package ui creates replykeyboard and catch her events
package ui

import (
	tele "gopkg.in/telebot.v3"
)

// MainMenu creates a replykeyboard with buttons.
func MainMenu(btnTexts []string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	currentRow := make([]tele.Btn, 0, 2)
	btns := make([]tele.Row, 0, len(btnTexts))

	for _, text := range btnTexts {
		btn := menu.Text(text)
		currentRow = append(currentRow, btn)

		if len(currentRow) == 2 {
			btns = append(btns, menu.Row(currentRow...))
			currentRow = currentRow[:0]
		}
	}

	if len(currentRow) > 0 {
		btns = append(btns, menu.Row(currentRow...))
	}

	menu.Reply(btns...)

	return menu
}
