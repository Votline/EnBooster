// Package router users.go handles user states (not admin)
package router

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
			if err := c.Send("Shiritpri game stopped. " + msg); err != nil {
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
			if err := c.Send("Chatting mode stopped."); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}
		reqTrace := uuid.NewString()
		res, err := srv.aisrv.GenerateText(c.Sender().ID, usrMsg, reqTrace)
		if err != nil {
			return fmt.Errorf("%s: generate text: %w", op, err)
		}
		if len(res) == 0 {
			if err := c.Send("AI didn't generate any text"); err != nil {
				return fmt.Errorf("%s: bot send: %w", op, err)
			}
			return nil
		}
		if err := c.Send(res); err != nil {
			return fmt.Errorf("%s: bot send: %w", op, err)
		}
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
