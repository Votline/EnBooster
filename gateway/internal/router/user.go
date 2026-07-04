// Package router users.go handles user states (not admin)
package router

import (
	"fmt"

	sm "enbstr/internal/statemanager"

	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)

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
		userAnswer := c.Message().Text
		answer := usrctx.Data
		if ok := srv.lrnsrv.VerifyAnswer(userAnswer, answer, reqTrace); !ok {
			err = c.Send(fmt.Sprintf("Incorrect answer. Correct answer: %s", answer))
		} else {
			err = c.Send("Correct answer")
		}
		setToNone = true
	default:
		setToNone = true
	}
	if setToNone {
		if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateNone, ""); err != nil {
			return fmt.Errorf("%s: set state: %w", op, err)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: failed to set state: %w", op, err)
	}

	return nil
}
