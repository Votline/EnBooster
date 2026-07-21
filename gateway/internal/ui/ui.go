// Package ui inline.go creates inline keyboard and catch her events
package ui

import tele "gopkg.in/telebot.v3"

// InlineBtn is a button for inline keyboard
type InlineBtn struct {
	Text string
	Data string
}

// UI contains all the keyboards for the bot
type UI struct {
	UserMain        *tele.ReplyMarkup
	AdminMain       *tele.ReplyMarkup
	AdminCommands   *tele.ReplyMarkup
	Stopmenu        *tele.ReplyMarkup
	UserSettings    *tele.ReplyMarkup
	TranscriptVoice *tele.ReplyMarkup
	AISettings      *tele.ReplyMarkup
}

const (
	AISystemPromptID  = "\fai_system_prompt"
	ChangeLangLevelID = "\fchange_lang_level"
	TranscriptVoiceID = "\ftranscript_voice"
)

// NewUI creates a new UI instance
func NewUI() *UI {
	usermain := replyMenu(2, []string{"Learning", "Shiritori", "Chatting", "Profile"})
	usersettings := inlineMenu(1, []InlineBtn{
		{"Set AI system prompt", AISystemPromptID[1:]},
		{"Set language level", ChangeLangLevelID[1:]},
	})
	transcriptVoice := inlineMenu(1, []InlineBtn{
		{"Transcript voice", TranscriptVoiceID[1:]},
	})
	adminmain := replyMenu(2, []string{"Learning", "Shiritori", "Chatting", "Profile", "Help"})
	admincmds := replyMenu(2, []string{"tasks_add", "task_del", "words_add", "word_del"})
	stopmenu := replyMenu(1, []string{"/stop"})
	aisettings := replyMenu(2, []string{"Use TTS", "Use STT", "Use STT And TTS", "Use TTT"})
	return &UI{
		UserMain:        usermain,
		UserSettings:    usersettings,
		AdminMain:       adminmain,
		AdminCommands:   admincmds,
		Stopmenu:        stopmenu,
		TranscriptVoice: transcriptVoice,
		AISettings:      aisettings,
	}
}

// replyMenu creates a replykeyboard with buttons.
func replyMenu(objectInRow int, btnTexts []string) *tele.ReplyMarkup {
	if len(btnTexts) == 0 {
		return nil
	}
	if objectInRow < 1 {
		objectInRow = 1
	}

	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btns := make([]tele.Row, 0, len(btnTexts))
	var currentRow []tele.Btn

	for _, text := range btnTexts {
		btn := menu.Text(text)
		currentRow = append(currentRow, btn)

		if len(currentRow) == objectInRow {
			btns = append(btns, menu.Row(currentRow...))
			currentRow = nil
		}
	}

	if len(currentRow) > 0 {
		btns = append(btns, menu.Row(currentRow...))
	}

	menu.Reply(btns...)

	return menu
}

// inlineMenu creates a inline keyboard with buttons.
func inlineMenu(objectInRow int, btnData []InlineBtn) *tele.ReplyMarkup {
	if len(btnData) == 0 {
		return nil
	}
	if objectInRow < 1 {
		objectInRow = 1
	}

	menu := &tele.ReplyMarkup{}

	var rows []tele.Row
	var currentRow []tele.Btn

	for _, btn := range btnData {
		btn := menu.Data(btn.Text, btn.Data)
		currentRow = append(currentRow, btn)

		if len(currentRow) == objectInRow {
			rows = append(rows, menu.Row(currentRow...))
			currentRow = nil
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, menu.Row(currentRow...))
	}

	menu.Inline(rows...)

	return menu
}
