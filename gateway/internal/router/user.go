// Package router users.go handles user states (not admin)
package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unsafe"

	"enbstr/internal/learn"
	sm "enbstr/internal/statemanager"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

var lastLetterPool = sync.Pool{
	New: func() any {
		var s string
		return &s
	},
}

var wordsPool = sync.Pool{
	New: func() any {
		var w []learn.Word
		return &w
	},
}

var aiTextPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

const updateInterval = time.Second * 1

// handleState check user state and call services
func (srv *Server) handleState(c tele.Context) error {
	const op = "router.handleState"

	usrctx, err := srv.sm.GetUserCtx(c.Sender().ID)
	if err != nil {
		return fmt.Errorf("%s: get user state: %w", op, err)
	}

	state := usrctx.State
	reqTrace := uuid.NewString()

	var setToNone bool
	switch state {
	case sm.StateTaskLearning:
		var taskSes sm.TaskSession
		uctxData := unsafe.Slice(unsafe.StringData(usrctx.JSONData), len(usrctx.JSONData))
		if err := json.Unmarshal(uctxData, &taskSes); err != nil {
			return fmt.Errorf("%s: unmarshal: %w", op, err)
		}

		add := 0
		answer := taskSes.Answer
		userAnswer := c.Message().Text
		correct := srv.lrnsrv.VerifyAnswer(userAnswer, answer, reqTrace)
		if !correct {
			add = -1
			err = c.Send(fmt.Sprintf("Incorrect answer. Correct answer: %s", answer))
		} else {
			add = 1
			err = c.Send("Correct answer")
		}
		if err := srv.usrsrv.UpdateUserTaskCtx(c.Sender().ID, sm.StateTaskLearning, taskSes.CurrentTheme, reqTrace, answer, add, srv.sm); err != nil {
			srv.log.Error("Failed to update user task ctx",
				zap.String("op", op),
				zap.String("reqTrace", reqTrace),
				zap.Error(err))
		}

		srv.usrsrv.UpdateByAnswer(c.Sender().ID, correct, taskSes.Counter+add, taskSes.CurrentTheme, reqTrace)
		if err := srv.usrsrv.UpdateUserTaskCtx(c.Sender().ID, sm.StateNone, taskSes.CurrentTheme, reqTrace, answer, add, srv.sm); err != nil {
			srv.log.Error("Failed to update user task ctx",
				zap.String("op", op),
				zap.String("reqTrace", reqTrace),
				zap.Error(err))
		}
	case sm.StateShiritori:
		userWord := c.Message().Text

		// unmarshal to structure
		usrctx, err := srv.sm.GetUserCtx(c.Sender().ID)
		if err != nil {
			return fmt.Errorf("%s: get user state: %w", op, err)
		}
		var shiritoriSes sm.ShiritoriSession
		if usrctx.JSONData != "" {
			uctxData := unsafe.Slice(unsafe.StringData(usrctx.JSONData), len(usrctx.JSONData))
			if err := json.Unmarshal(uctxData, &shiritoriSes); err != nil {
				return fmt.Errorf("%s: unmarshal: %w", op, err)
			}
		}

		// check if user wants to stop the game
		// print shiritori statistics
		if strings.ToLower(userWord) == "/stop" {
			if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateNone, nil); err != nil {
				return fmt.Errorf("%s: change state: %w", op, err)
			}

			botWords := shiritoriSes.AllWords - shiritoriSes.UserWords
			incorrectWords := shiritoriSes.UserWords - shiritoriSes.UserCorrectWords

			msg := fmt.Sprintf(
				"Shiritori game stopped.\n\n"+
					"Match Statistics: \n"+
					"Total words in game: %d\n"+
					"Bot words: %d\n"+
					"Your total attempts: %d\n"+
					"Your correct words: %d\n"+
					"Your mistakes: %d\n",
				shiritoriSes.AllWords,
				botWords,
				shiritoriSes.UserWords,
				shiritoriSes.UserCorrectWords,
				incorrectWords,
			)
			if err := c.Send("Shiritpri game stopped. "+msg, srv.uiInstns.UserMain); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}

		// increment all words and user words
		shiritoriSes.AllWords += 1
		shiritoriSes.UserWords += 1

		// get last letter of the user word
		lastLetter := lastLetterPool.Get().(*string)
		defer lastLetterPool.Put(lastLetter)

		srv.lrnsrv.GetLastLetter(userWord, lastLetter)
		if *lastLetter == "" {
			return c.Send("Invalid word")
		}

		// get offset of the last letter
		if shiritoriSes.LetterOffsets == nil {
			shiritoriSes.LetterOffsets = make(map[string]int)
		}
		offsetID := shiritoriSes.LetterOffsets[*lastLetter]

		// get words with target
		wordsPtr := wordsPool.Get().(*[]learn.Word)
		defer wordsPool.Put(wordsPtr)

		found, err := srv.lrnsrv.GetWordsWithTarget(userWord, *lastLetter, reqTrace, offsetID, wordsPtr)
		if err != nil {
			return fmt.Errorf("%s: get words: %w", op, err)
		}

		// check if bot found any word and user word exists
		// AND increment all words counter
		if !found {
			if err := srv.saveState(c, shiritoriSes); err != nil {
				return fmt.Errorf("%s: save state: %w", op, err)
			}
			if err := c.Send("Your word not found"); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}

		if len(*wordsPtr) == 0 {
			if err := srv.saveState(c, shiritoriSes); err != nil {
				return fmt.Errorf("%s: save state: %w", op, err)
			}
			if err := c.Send("Bot couldn't find any word"); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		} else {
			shiritoriSes.AllWords += 1
		}

		botWord := (*wordsPtr)[0].Word
		botWordID := (*wordsPtr)[0].Serial

		botLastLetter := lastLetterPool.Get().(*string)
		defer lastLetterPool.Put(botLastLetter)

		srv.lrnsrv.GetLastLetter(botWord, botLastLetter)
		if *botLastLetter == "" {
			return fmt.Errorf("%s: get last letter bot word: %s: %w", op, botWord, err)
		}

		// update user shiritori ctx with get bools for shiritori rules
		// inside the method also increment user CORRECT words
		repeat, notMatch, err := srv.usrsrv.UpdateUserShiritoriCtx(
			&shiritoriSes, c.Sender().ID, userWord, *lastLetter, botWord, *botLastLetter,
			botWordID, sm.StateShiritori, srv.sm)
		if err != nil {
			return fmt.Errorf("%s: update user shiritori ctx: %w", op, err)
		}

		// shiritori rules

		if repeat {
			if err := c.Send("You already used this word"); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}

		if notMatch {
			if err := c.Send("First letter in your word doesn't match with last letter in previous word"); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}

		if err := c.Send(fmt.Sprintf("Word:\n%v", botWord)); err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}
	case sm.StateChatting:
		usrMsg := c.Message().Text
		if usrMsg == "/stop" {
			if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateNone, nil); err != nil {
				return fmt.Errorf("%s: change state: %w", op, err)
			}
			if err := c.Send("Chatting mode stopped.", srv.uiInstns.UserMain); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}

		msg, err := c.Bot().Send(c.Recipient(), "AI is generating text...")
		if err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}

		resBuf := aiTextPool.Get().(*bytes.Buffer)
		resBuf.Reset()
		defer aiTextPool.Put(resBuf)

		var lastUpdate time.Time
		reqTrace := uuid.NewString()

		if err := srv.generateText(c, usrMsg, reqTrace, func(text string) {
			resBuf.WriteString(text)

			if time.Since(lastUpdate) > updateInterval {
				lastUpdate = time.Now()
				if _, err := c.Bot().Edit(msg, resBuf.String()); err != nil {
					srv.log.Error("Failed to edit message",
						zap.String("op", op),
						zap.String("request_trace", reqTrace),
						zap.Error(err))
				}
			}
		}); err != nil {
			return fmt.Errorf("%s: generate text: %w", op, err)
		}

	case sm.StateTTS:
		usrMsg := c.Message().Text
		if usrMsg == "/stop" {
			if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateNone, nil); err != nil {
				return fmt.Errorf("%s: change state: %w", op, err)
			}
			if err := c.Send("TTS mode stopped.", srv.uiInstns.UserMain); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}

		statusMsg, err := c.Bot().Send(c.Recipient(), "AI is generating audio...")
		if err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}

		resBuf := aiTextPool.Get().(*bytes.Buffer)
		resBuf.Reset()
		defer aiTextPool.Put(resBuf)

		if err := srv.generateText(c, usrMsg, reqTrace, func(text string) {
			resBuf.WriteString(text)
		}); err != nil {
			return fmt.Errorf("%s: generate text: %w", op, err)
		}

		generatedText := resBuf.String()
		resBuf.Reset()

		if err := srv.aisrv.GenerateAudio(generatedText, reqTrace, func(audio []byte) {
			resBuf.Write(audio)
		}); err != nil {
			return fmt.Errorf("%s: generate audio: %w", op, err)
		}

		voiceMsg := &tele.Voice{
			File: tele.FromReader(bytes.NewReader(resBuf.Bytes())),
		}

		if err := c.Bot().Delete(statusMsg); err != nil {
			srv.log.Error("Failed to delete message",
				zap.String("op", op),
				zap.String("request_trace", reqTrace),
				zap.Error(err))
		}

		if err := c.Send(voiceMsg, srv.uiInstns.TranscriptVoice); err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}
	case sm.StateSTT:
		voiceMsg := c.Message().Voice
		if voiceMsg == nil {
			usrMsg := c.Message().Text
			if usrMsg == "/stop" {
				if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateNone, nil); err != nil {
					return fmt.Errorf("%s: change state: %w", op, err)
				}
				if err := c.Send("STT mode stopped.", srv.uiInstns.UserMain); err != nil {
					return fmt.Errorf("%s: bot send: %w", op, err)
				}
				return nil
			} else {
				return c.Send("Invalid voice message")
			}
		}

		reader, err := c.Bot().File(&voiceMsg.File)
		if err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}

		oggBytes, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("%s: read ogg file: %w", op, err)
		}

		statusMsg, err := c.Bot().Send(c.Recipient(), "AI is recognizing audio...")
		if err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}

		srv.log.Debug("Recognize audio request",
			zap.String("op", op),
			zap.String("request_trace", reqTrace))

		var lastUpdate time.Time
		if err := srv.aisrv.RecognizeAudio(oggBytes, reqTrace, func(text string) {
			if time.Since(lastUpdate) > updateInterval {
				lastUpdate = time.Now()
				if _, err := c.Bot().Edit(statusMsg, text); err != nil {
					srv.log.Error("Failed to edit message",
						zap.String("op", op),
						zap.String("request_trace", reqTrace),
						zap.Error(err))
				}
			}
		}); err != nil {
			return fmt.Errorf("%s: generate text: %w", op, err)
		}

		srv.log.Debug("Recognize audio successfully",
			zap.String("op", op),
			zap.String("request_trace", reqTrace))

	case sm.StateSetSysPrompt:
		srv.log.Debug("Change system prompt",
			zap.String("op", op),
			zap.String("request_trace", reqTrace))

		sp := c.Message().Text
		if err := srv.usrsrv.UpdSystemPrompt(c.Sender().ID, sp, reqTrace); err != nil {
			return fmt.Errorf("%s: update system prompt: %w", op, err)
		}
		if err := c.Send("System prompt updated"); err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}

		srv.log.Debug("Successfully changed system prompt",
			zap.String("op", op),
			zap.String("request_trace", reqTrace))

		setToNone = true
	case sm.StateSetLangLevel:
		srv.log.Debug("Change language level",
			zap.String("op", op),
			zap.String("request_trace", reqTrace))

		lvl := c.Message().Text
		if err := srv.usrsrv.UpdLangLevel(c.Sender().ID, lvl, reqTrace); err != nil {
			return fmt.Errorf("%s: update language level: %w", op, err)
		}

		if err := c.Send("Level updated"); err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}

		srv.log.Debug("Successfully changed language level",
			zap.String("op", op),
			zap.String("request_trace", reqTrace))

		setToNone = true
	default:
		setToNone = true
	}
	if setToNone {
		if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateNone, nil); err != nil {
			return fmt.Errorf("%s: set state: %w", op, err)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: failed to set state: %w", op, err)
	}

	return nil
}

func (srv *Server) saveState(c tele.Context, shiritoriSes sm.ShiritoriSession) error {
	const op = "router.user.saveState"

	jsonData, err := json.Marshal(shiritoriSes)
	if err != nil {
		return fmt.Errorf("%s: marshal json: %w", op, err)
	}
	return srv.sm.SetUserCtx(c.Sender().ID, sm.StateShiritori, jsonData)
}

// generateText generates a text message from the given user message and
// yields the result to the given callback function.
func (srv *Server) generateText(c tele.Context, usrMsg, reqTrace string, yield func(text string)) error {
	const op = "router.user.generateText"

	aiSes, err := srv.sm.GetUserCtx(c.Sender().ID)
	if err != nil {
		return fmt.Errorf("%s: get user state: %w", op, err)
	}
	var chatSes sm.ChattingSession
	if len(aiSes.JSONData) > 0 {
		uctxData := unsafe.Slice(unsafe.StringData(aiSes.JSONData), len(aiSes.JSONData))
		if err := json.Unmarshal(uctxData, &chatSes); err != nil {
			return fmt.Errorf("%s: unmarshal: %w", op, err)
		}
	}
	sysPrompt := chatSes.SystemPrompt

	var builder strings.Builder
	if err := srv.aisrv.GenerateText(c.Sender().ID, usrMsg, sysPrompt, reqTrace, func(res []byte) {
		resStr := unsafe.String(unsafe.SliceData(res), len(res))
		builder.WriteString(resStr)
		yield(resStr)

		srv.log.Debug("Save bot msg", zap.String("str", resStr))
	}); err != nil {
		return fmt.Errorf("%s: generate text: %w", op, err)
	}

	chatSes.LastMessage = builder.String()

	jsonData, err := json.Marshal(chatSes)
	if err != nil {
		return fmt.Errorf("%s: marshal json: %w", op, err)
	}

	if err := srv.sm.UpdateUserDataCtx(c.Sender().ID, jsonData); err != nil {
		return fmt.Errorf("%s: update user data: %w", op, err)
	}

	return nil
}
