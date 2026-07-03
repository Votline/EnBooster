// Package router admin.go handle admin commands
package router

import (
	"fmt"

	sm "enbstr/internal/statemanager"

	tele "gopkg.in/telebot.v3"
)

func (srv *Server) handleAdmin(c tele.Context) error {
	const op = "router.admin"

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
			err = c.Send("Задание добавлено")
		case sm.StateTaskDeleting:
			err = c.Send("Задание удалено")
		case sm.StateWordAdding:
			err = c.Send("Слово добавлено")
		case sm.StateWordDeleting:
			err = c.Send("Слово удалено")
		}
	}
	if err != nil {
		return fmt.Errorf("%s: failed to set state: %w", op, err)
	}
	return nil
}
