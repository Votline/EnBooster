// Package router admin.go handle admin commands
package router

import (
	"fmt"

	sm "enbstr/internal/statemanager"
	"enbstr/internal/ui"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

const helpMsg = `
* task_add - add task. Format json:
> [{"task":" <full task message> ","level":"<english level>","answer": "<answer(s)>" }]
* task_del - delete task. Format message:
> <level> <position>
* word_add - add word. Format json:
> [{"word":"<word>","explain":"<explain>","level":"<english level>","first_letter":"<first letter>"}]
* word_del - delete word. Format message:
> <word> <serial number>
`

func (srv *Server) handleAdmin(c tele.Context) error {
	const op = "router.admin"

	srv.log.Debug("Admin command received",
		zap.String("op", op))

	reqTrace := uuid.NewString()

	var err error
	var state int8
	setToNone := false
	switch c.Message().Text {
	case "Help":
		helpMenu := ui.ReplyMenu([]string{"task_add", "task_del", "word_add", "word_del"})
		err = c.Send(helpMsg, helpMenu)
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
			setToNone = true
		case sm.StateTaskDeleting:
			if err = srv.lrnsrv.DelTask(c.Message().Text, reqTrace); err != nil {
				return fmt.Errorf("%s: delete task: %w", op, err)
			}
			err = c.Send("Task deleted")
			setToNone = true
		case sm.StateWordAdding:
			var inserted int32
			inserted, err = srv.lrnsrv.NewWords(c.Message().Text, reqTrace)
			if err != nil {
				return fmt.Errorf("%s: add word: %w", op, err)
			}
			err = c.Send(fmt.Sprintf("Inserted words: %d", inserted))
			setToNone = true
		case sm.StateWordDeleting:
			if err = srv.lrnsrv.DelWord(c.Message().Text, reqTrace); err != nil {
				return fmt.Errorf("%s: delete word: %w", op, err)
			}
			err = c.Send("Word deleted")
			setToNone = true
		default:
			err = c.Send("Unknown command")
			setToNone = true
		}
	}
	if setToNone {
		if err := srv.sm.SetState(c.Message().Sender.ID, sm.StateNone); err != nil {
			return fmt.Errorf("%s: set state: %w", op, err)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: failed to set state: %w", op, err)
	}
	return nil
}
