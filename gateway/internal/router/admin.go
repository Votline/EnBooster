// Package router admin.go handle admin commands
package router

import (
	"fmt"

	sm "enbstr/internal/statemanager"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

// helpMsg is a help message for admin commands
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

// handleAdmin handles admin commands and
// changes the state of the admin user
func (srv *Server) handleAdmin(c tele.Context) error {
	const op = "router.admin"

	srv.log.Debug("Admin command received",
		zap.String("op", op))

	reqTrace := uuid.NewString()

	var err error
	var usrctx sm.UserContext
	setToNone := false
	switch c.Message().Text {
	case "Help":
		err = c.Send(helpMsg, srv.uiInstns.AdminCommands)
	case "tasks_add":
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateTaskAdding, nil)
	case "task_del":
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateTaskDeleting, nil)
	case "words_add":
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateWordAdding, nil)
	case "word_del":
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateWordDeleting, nil)
	default:
		usrctx, err = srv.sm.GetUserCtx(c.Sender().ID)
		if err != nil {
			return fmt.Errorf("%s: get state: %w", op, err)
		}
		state := usrctx.State
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
			setToNone = true
		}
	}
	if setToNone {
		if err := srv.sm.SetUserCtx(c.Message().Sender.ID, sm.StateNone, nil); err != nil {
			return fmt.Errorf("%s: set state: %w", op, err)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: failed to set state: %w", op, err)
	}
	return nil
}
