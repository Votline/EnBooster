// Package router admin.go handle admin commands
package router

import (
	"fmt"

	sm "enbstr/internal/statemanager"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

func (srv *Server) handleAdmin(c tele.Context) error {
	const op = "router.admin"

	srv.log.Debug("Admin command received",
		zap.String("op", op))

	reqTrace := uuid.NewString()

	var err error
	var state int8
	switch c.Message().Text {
	case "task_add":
		err = srv.sm.SetState(c.Sender().ID, sm.StateTaskAdding)
	case "task_del":
		err = srv.sm.SetState(c.Sender().ID, sm.StateTaskDeleting)
	case "word_add":
		err = srv.sm.SetState(c.Sender().ID, sm.StateWordAdding)
	case "word_del":
		err = srv.sm.SetState(c.Sender().ID, sm.StateWordDeleting)
	default:
		state, err = srv.sm.GetState(c.Sender().ID)
		if err != nil {
			return fmt.Errorf("%s: get state: %w", op, err)
		}
		switch state {
		case sm.StateTaskAdding:
			var inserted int32
			inserted, err = srv.lrnsrv.NewTasks(c.Message().Text, reqTrace)
			if err != nil {
				return fmt.Errorf("%s: add task: %w", op, err)
			}
			err = c.Send(fmt.Sprintf("Inserted tasks: %d", inserted))
		case sm.StateTaskDeleting:
			if err = srv.lrnsrv.DelTask(c.Message().Text, reqTrace); err != nil {
				return fmt.Errorf("%s: delete task: %w", op, err)
			}
			err = c.Send("Task deleted")
		case sm.StateWordAdding:
			var inserted int32
			inserted, err = srv.lrnsrv.NewWords(c.Message().Text, reqTrace)
			if err != nil {
				return fmt.Errorf("%s: add word: %w", op, err)
			}
			err = c.Send(fmt.Sprintf("Inserted words: %d", inserted))
		case sm.StateWordDeleting:
			if err = srv.lrnsrv.DelWord(c.Message().Text, reqTrace); err != nil {
				return fmt.Errorf("%s: delete word: %w", op, err)
			}
			err = c.Send("Word deleted")
		default:
			err = c.Send("Unknown command")
		}
	}
	if err != nil {
		return fmt.Errorf("%s: failed to set state: %w", op, err)
	}
	return nil
}
