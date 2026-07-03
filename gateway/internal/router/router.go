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
	mdwrs := make([]middlewares.Middleware, 0, 1)
	setupMiddlewares(&mdwrs, log)
	srv := &Server{
		b:        bot,
		log:      log,
		closable: make([]services.Closable, 0, 2),
		mdwrs:    mdwrs,
	}

	redisCtxTimeout := GetEnvInt("RedisCtxTimeout", 10)
	stateTTL := GetEnvInt("StateTTL", 30)
	pingTimeout := GetEnvInt("RedisPingTimeout", 10)
	ctxTimeout := GetEnvInt("CtxTimeout", 10)

	states, err := statemanager.NewSM(time.Duration(redisCtxTimeout), time.Duration(stateTTL), time.Duration(pingTimeout))
	if err != nil {
		log.Fatal("Failed to create state manager", zap.Error(err))
	}
	log.Info("Successfully connected redis")

	srv.setupServices(bot, states, time.Duration(ctxTimeout), log)
	srv.ctxTimeout = time.Duration(ctxTimeout)

	return srv
}

func setupMiddlewares(mdwrs *[]middlewares.Middleware, log *zap.Logger) {
	const op = "router.setupMiddlewares"
	rl := middlewares.NewRateLimiter()
	*mdwrs = append(*mdwrs, rl)
	log.Debug("Added rate limiter middleware", zap.String("op", op))
}

func (srv *Server) setupServices(bot *tele.Bot, states *statemanager.StateManager, ctxTimeout time.Duration, log *zap.Logger) {
	usrsrv, err := users.NewUS(ctxTimeout, log)
	if err != nil {
		log.Fatal("Failed to create users service", zap.Error(err))
	}

	lrnsrv, err := learn.NewLS(states, ctxTimeout, log)
	if err != nil {
		log.Fatal("Failed to create learn service", zap.Error(err))
	}

	bot.Handle(tele.OnText, func(c tele.Context) error {
		msg := c.Message()

		ctx, cancel := context.WithTimeout(context.Background(), srv.ctxTimeout)
		defer cancel()

		for _, mdwr := range srv.mdwrs {
			if err := mdwr.Handle(ctx); err != nil {
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
