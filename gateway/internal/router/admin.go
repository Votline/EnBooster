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
* tasks_add - add task. Format json:
> [{"task":" <full task message> ","level":"<english level>","answer": "<answer(s)>" }]
* task_del - delete task. Format message:
> <level> <position>
* words_add - add word. Format json:
> [{"word":"<word>","explain":"<explain>","level":"<english level>","first_letter":"<first letter>"}]
* word_del - delete word. Format message:
> <word> <serial number>
`

// handleAdmin handles admin commands and
// changes the state of the admin user
// returns error and the command or state was handled
func (srv *Server) handleAdmin(c tele.Context) (bool, error) {
	const op = "router.admin"

	srv.log.Debug("Admin command received",
		zap.String("op", op))

	reqTrace := uuid.NewString()

	var err error
	handled := false
	var usrctx sm.UserContext
	setToNone := false
	switch c.Message().Text {
	case "Help":
		handled = true
		err = c.Send(helpMsg, srv.uiInstns.AdminCommands)
	case "tasks_add":
		handled = true
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateTaskAdding, nil)
	case "task_del":
		handled = true
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateTaskDeleting, nil)
	case "words_add":
		handled = true
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateWordAdding, nil)
		handled = true
	case "word_del":
		err = srv.sm.SetUserCtx(c.Sender().ID, sm.StateWordDeleting, nil)
	default:
		usrctx, err = srv.sm.GetUserCtx(c.Sender().ID)
		if err != nil {
			return handled, fmt.Errorf("%s: get state: %w", op, err)
		}
		state := usrctx.State
		switch state {
		case sm.StateTaskAdding:
			handled = true
			var inserted int32
			inserted, err = srv.lrnsrv.NewTasks(c.Message().Text, reqTrace)
			if err != nil {
				return handled, fmt.Errorf("%s: add task: %w", op, err)
			}
			err = c.Send(fmt.Sprintf("Inserted tasks: %d", inserted))
			setToNone = true
		case sm.StateTaskDeleting:
			handled = true
			if err = srv.lrnsrv.DelTask(c.Message().Text, reqTrace); err != nil {
				return handled, fmt.Errorf("%s: delete task: %w", op, err)
			}
			err = c.Send("Task deleted")
			setToNone = true
		case sm.StateWordAdding:
			handled = true
			var inserted int32
			inserted, err = srv.lrnsrv.NewWords(c.Message().Text, reqTrace)
			if err != nil {
				return handled, fmt.Errorf("%s: add word: %w", op, err)
			}
			err = c.Send(fmt.Sprintf("Inserted words: %d", inserted))
			setToNone = true
		case sm.StateWordDeleting:
			handled = true
			if err = srv.lrnsrv.DelWord(c.Message().Text, reqTrace); err != nil {
				return handled, fmt.Errorf("%s: delete word: %w", op, err)
			}
			err = c.Send("Word deleted")
			setToNone = true
		}
	}
	if setToNone {
		if err := srv.sm.SetUserCtx(c.Message().Sender.ID, sm.StateNone, nil); err != nil {
			return handled, fmt.Errorf("%s: set state: %w", op, err)
		}
	}
	if err != nil {
		return handled, fmt.Errorf("%s: failed to set state: %w", op, err)
	}
	return handled, nil
}
