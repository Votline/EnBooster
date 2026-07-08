// Package router handles user messages and calls handlers
package router

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"enbstr/internal/learn"
	"enbstr/internal/middlewares"
	"enbstr/internal/services"
	sm "enbstr/internal/statemanager"
	"enbstr/internal/users"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

// Server is a struct for managing services, middlewares and bot
type Server struct {
	adminUUID  int64
	ctxTimeout time.Duration
	b          *tele.Bot
	log        *zap.Logger
	closable   []services.Closable
	mdwrs      []middlewares.Middleware
	sm         *sm.StateManager

	usrsrv *users.UsersService
	lrnsrv *learn.LearnService
}

// tasksList is a sync.Pool for tasks
var tasksList = sync.Pool{
	New: func() any {
		l := make([]learn.Task, 0)
		return &l
	},
}

// GetEnvInt returns an environment variable as an integer
func GetEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// Setup creates a new Server instance
// with created services and middlewares
func Setup(bot *tele.Bot, log *zap.Logger) *Server {
	srv := &Server{
		b:        bot,
		log:      log,
		closable: make([]services.Closable, 0, 2),
		mdwrs:    make([]middlewares.Middleware, 0, 1),
	}

	redisCtxTimeout := time.Duration(GetEnvInt("REDIS_CTX_TIMEOUT", 10))
	stateTTL := time.Duration(GetEnvInt("STATE_TTL", 30))
	ratelimitTTL := time.Duration(GetEnvInt("RATE_LIMIT_TTL", 30))
	pingTimeout := time.Duration(GetEnvInt("REDIS_PING_TIMEOUT", 10))
	ctxTimeout := time.Duration(GetEnvInt("CTX_TIMEOUT", 10))
	adminUUID := int64(GetEnvInt("ADMIN_UUID", 0))

	states, err := sm.NewSM(redisCtxTimeout, stateTTL, pingTimeout)
	if err != nil {
		log.Fatal("Failed to create state manager", zap.Error(err))
	}
	log.Info("Successfully connected redis state manager")

	srv.ctxTimeout = time.Duration(ctxTimeout)
	srv.adminUUID = adminUUID
	srv.sm = states

	srv.setupMiddlewares(redisCtxTimeout, ratelimitTTL, pingTimeout)
	srv.setupServices()
	srv.handleMessages(bot)

	return srv
}

// setupMiddlewares creates middlewares
func (srv *Server) setupMiddlewares(ctxTimout, rlTTL, pingTimeout time.Duration) {
	const op = "router.setupMiddlewares"

	rl, err := middlewares.NewRateLimiter(ctxTimout, rlTTL, pingTimeout)
	if err != nil {
		srv.log.Fatal("Failed to create rate limiter middleware",
			zap.Error(err))
	}
	srv.mdwrs = append(srv.mdwrs, rl)

	srv.log.Debug("Added rate limiter middleware", zap.String("op", op))
}

// setupServices creates services and appends them to closable
func (srv *Server) setupServices() {
	usrsrv, err := users.NewUS(srv.ctxTimeout, srv.adminUUID, srv.log)
	if err != nil {
		srv.log.Fatal("Failed to create users service", zap.Error(err))
	}
	srv.usrsrv = usrsrv

	lrnsrv, err := learn.NewLS(srv.sm, srv.ctxTimeout, srv.log)
	if err != nil {
		srv.log.Fatal("Failed to create learn service", zap.Error(err))
	}
	srv.lrnsrv = lrnsrv

	srv.closable = append(srv.closable, usrsrv, lrnsrv)
}

// handleMessages handles users messages
// call middlewares for each message
// and call services handlers
func (srv *Server) handleMessages(bot *tele.Bot) {
	const op = "router.handleMessages"

	bot.Handle(tele.OnText, func(c tele.Context) error {
		msg := c.Message()

		ctx, cancel := context.WithTimeout(context.Background(), srv.ctxTimeout*time.Second)
		defer cancel()

		for _, mdwr := range srv.mdwrs {
			if err := mdwr.Handle(ctx, c); err != nil {
				return fmt.Errorf("%s: middleware: %w", op, err)
			}
		}

		switch msg.Text {
		case "/start", "Profile":
			srv.usrsrv.HandleRoutes(msg.Text, c)
		case "Learning":
			reqTrace := uuid.NewString()
			data, err := srv.usrsrv.GetData(c.Sender().ID, reqTrace)
			if err != nil {
				return fmt.Errorf("%s: get user data: %w", op, err)
			}

			tasksPtr := tasksList.Get().(*[]learn.Task)
			defer tasksList.Put(tasksPtr)

			if err := srv.lrnsrv.GetTasks(data.Level, data.TaskID, 1, tasksPtr, reqTrace); err != nil {
				return fmt.Errorf("%s: get tasks: %w", op, err)
			}

			if len(*tasksPtr) == 0 {
				return c.Send("Task not found")
			}

			answer := (*tasksPtr)[0].Answer
			theme := (*tasksPtr)[0].Theme

			if err := srv.usrsrv.UpdateUserTaskCtx(c.Sender().ID, sm.StateTaskLearning, theme, reqTrace, answer, 0, srv.sm); err != nil {
				return fmt.Errorf("%s: update user task ctx: %w", op, err)
			}

			return c.Send(fmt.Sprintf("Task:\n%v", (*tasksPtr)[0]))
		case "Shiritori":
			if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateShiritori, nil); err != nil {
				return fmt.Errorf("%s: set state: %w", op, err)
			}
			return c.Send("Shiritori mode activated. To exit push '/stop' button. \nWrite any word")
		default:
			usrctx, err := srv.sm.GetUserCtx(c.Sender().ID)
			if err != nil {
				return fmt.Errorf("%s: get user state: %w", op, err)
			}
			if c.Sender().ID == srv.adminUUID && usrctx.State != sm.StateAdminNotCommand {
				return srv.handleAdmin(c)
			}
			return srv.handleState(c)
		}
		return nil
	})
}

// Start starts the bot
func (srv *Server) Start() {
	srv.b.Start()
}

// Close closes the bot and services
func (srv *Server) Close() {
	const op = "router.Close"

	for _, c := range srv.closable {
		c.Close()
		srv.log.Info("Closed service",
			zap.String("service", c.GetName()),
			zap.String("op", op))
	}
	srv.b.Stop()
	srv.log.Info("Bot stopped", zap.String("op", op))
}
