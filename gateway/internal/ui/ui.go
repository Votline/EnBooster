// Package ui inline.go creates inline keyboard and catch her events
package ui

import tele "gopkg.in/telebot.v3"

// InlineBtn is a button for inline keyboard
type InlineBtn struct {
	Text string
	Data string
}

// ReplyMenu creates a replykeyboard with buttons.
func ReplyMenu(btnTexts []string) *tele.ReplyMarkup {
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

// InlineMenu creates a inline keyboard with buttons.
func InlineMenu(btnData []InlineBtn) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}

	var rows []tele.Row
	var currentRow []tele.Btn

	for _, btn := range btnData {
		btn := menu.Data(btn.Text, btn.Data)
		currentRow = append(currentRow, btn)

		if len(currentRow) == 2 {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = currentRow[:0]
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	menu.Inline(rows...)

	return menu
}
