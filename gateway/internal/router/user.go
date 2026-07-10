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

		if strings.ToLower(userWord) == "/stop" {
			if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateNone, nil); err != nil {
				return fmt.Errorf("%s: change state: %w", op, err)
			}
			return nil
		}

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

		lastLetter := lastLetterPool.Get().(*string)
		defer lastLetterPool.Put(lastLetter)

		srv.lrnsrv.GetLastLetter(userWord, lastLetter)
		if *lastLetter == "" {
			return c.Send("Invalid word")
		}

		if shiritoriSes.LetterOffsets == nil {
			shiritoriSes.LetterOffsets = make(map[string]int)
		}
		offsetID := shiritoriSes.LetterOffsets[*lastLetter]

		wordsPtr := wordsPool.Get().(*[]learn.Word)
		defer wordsPool.Put(wordsPtr)

		found, err := srv.lrnsrv.GetWordsWithTarget(userWord, *lastLetter, reqTrace, offsetID, wordsPtr)
		if err != nil {
			return fmt.Errorf("%s: get words: %w", op, err)
		}
		if !found {
			return c.Send("Your word not found")
		}

		if len(*wordsPtr) == 0 {
			return c.Send("Bot couldn't find any word")
		}

		botWord := (*wordsPtr)[0].Word
		botWordID := (*wordsPtr)[0].Serial

		botLastLetter := lastLetterPool.Get().(*string)
		defer lastLetterPool.Put(botLastLetter)

		srv.lrnsrv.GetLastLetter(botWord, botLastLetter)
		if *botLastLetter == "" {
			return c.Send("Bot couldn't find any word")
		}

		repeat, notMatch, err := srv.usrsrv.UpdateUserShiritoriCtx(
			&shiritoriSes, c.Sender().ID, userWord, *lastLetter, botWord, *botLastLetter,
			botWordID, sm.StateShiritori, srv.sm)
		if err != nil {
			return fmt.Errorf("%s: update user shiritori ctx: %w", op, err)
		}
		if repeat {
			return c.Send("You already used this word")
		}
		if notMatch {
			return c.Send("First letter in your word doesn't match with last letter in previous word")
		}

		return c.Send(fmt.Sprintf("Word:\n%v", botWord))
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
