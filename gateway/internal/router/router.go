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
	"enbstr/internal/statemanager"
	"enbstr/internal/users"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type Server struct {
	ctxTimeout time.Duration
	b          *tele.Bot
	log        *zap.Logger
	closable   []services.Closable
	mdwrs      []middlewares.Middleware
	sm         *statemanager.StateManager
	adminUUID  int64
}

var tasksList = sync.Pool{
	New: func() any {
		l := make([]learn.Task, 0)
		return &l
	},
}

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

func Setup(bot *tele.Bot, log *zap.Logger) *Server {
	srv := &Server{
		b:        bot,
		log:      log,
		closable: make([]services.Closable, 0, 2),
		mdwrs:    make([]middlewares.Middleware, 0, 1),
	}

	redisCtxTimeout := time.Duration(GetEnvInt("RedisCtxTimeout", 10))
	stateTTL := time.Duration(GetEnvInt("StateTTL", 30))
	ratelimitTTL := time.Duration(GetEnvInt("RateLimitTTL", 30))
	pingTimeout := time.Duration(GetEnvInt("RedisPingTimeout", 10))
	ctxTimeout := time.Duration(GetEnvInt("CtxTimeout", 10))
	adminUUID := int64(GetEnvInt("ADMIN_UUID", 0))

	states, err := statemanager.NewSM(redisCtxTimeout, stateTTL, pingTimeout)
	if err != nil {
		log.Fatal("Failed to create state manager", zap.Error(err))
	}
	log.Info("Successfully connected redis state manager")

	srv.ctxTimeout = time.Duration(ctxTimeout)
	srv.adminUUID = adminUUID
	srv.sm = states

	srv.setupMiddlewares(redisCtxTimeout, ratelimitTTL, pingTimeout)
	srv.setupServices(bot, log)

	return srv
}

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

func (srv *Server) setupServices(bot *tele.Bot, log *zap.Logger) {
	usrsrv, err := users.NewUS(srv.ctxTimeout, log)
	if err != nil {
		log.Fatal("Failed to create users service", zap.Error(err))
	}

	lrnsrv, err := learn.NewLS(srv.sm, srv.ctxTimeout, log)
	if err != nil {
		log.Fatal("Failed to create learn service", zap.Error(err))
	}

	bot.Handle(tele.OnText, func(c tele.Context) error {
		msg := c.Message()

		ctx, cancel := context.WithTimeout(context.Background(), srv.ctxTimeout*time.Second)
		defer cancel()

		for _, mdwr := range srv.mdwrs {
			if err := mdwr.Handle(ctx, c); err != nil {
				return fmt.Errorf("middleware: %w", err)
			}
		}

		switch msg.Text {
		case "/start", "Profile":
			usrsrv.HandleRoutes(msg.Text, c)
		case "Learning":
			reqTrace := uuid.NewString()
			data, err := usrsrv.GetData(c.Sender().ID, reqTrace)
			if err != nil {
				return fmt.Errorf("get user data: %w", err)
			}

			tasksPtr := tasksList.Get().(*[]learn.Task)
			defer tasksList.Put(tasksPtr)

			if err := lrnsrv.GetTasks(data.Level, data.TaskID, tasksPtr, reqTrace); err != nil {
				return fmt.Errorf("get tasks: %w", err)
			}

			return c.Send(fmt.Sprintf("Список заданий:\n%v", *tasksPtr))
		default:
			if c.Sender().ID == srv.adminUUID {
				return srv.handleAdmin(c)
			}
		}
		return nil
	})

	srv.closable = append(srv.closable, usrsrv, lrnsrv)
}

func (srv *Server) Start() {
	srv.b.Start()
}

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
