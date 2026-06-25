// Package ui inline.go creates inline keyboard and catch her events
package ui

import tele "gopkg.in/telebot.v3"

// InlineBtn is a button for inline keyboard
type InlineBtn struct {
	Text string
	Data string
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
