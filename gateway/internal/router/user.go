// Package router users.go handles user states (not admin)
package router

import (
	"encoding/json"
	"fmt"
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
		lastLetter := lastLetterPool.Get().(*string)
		defer lastLetterPool.Put(lastLetter)

		userWord := c.Message().Text

		srv.lrnsrv.GetLastLetter(userWord, lastLetter)
		if *lastLetter == "" {
			return c.Send("Invalid word")
		}

		wordsPtr := wordsPool.Get().(*[]learn.Word)
		defer wordsPool.Put(wordsPtr)

		if err := srv.lrnsrv.GetWords(*lastLetter, reqTrace, wordsPtr); err != nil {
			return fmt.Errorf("%s: get words: %w", op, err)
		}

		if len(*wordsPtr) == 0 {
			return c.Send("Word not found")
		}

		return c.Send(fmt.Sprintf("Word:\n%v", (*wordsPtr)[0]))
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
