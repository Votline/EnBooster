// Package router handles user messages and calls handlers
package router

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"enbstr/internal/ai"
	"enbstr/internal/learn"
	"enbstr/internal/middlewares"
	"enbstr/internal/services"
	sm "enbstr/internal/statemanager"
	"enbstr/internal/ui"
	"enbstr/internal/users"

	"github.com/google/uuid"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

// Server is a struct for managing services, middlewares and bot
type Server struct {
	adminUUID  int64
	ctxTimeout time.Duration
	uiInstns   *ui.UI
	b          *tele.Bot
	log        *zap.Logger
	closable   []services.Closable
	mdwrs      []middlewares.Middleware
	sm         *sm.StateManager

	usrsrv *users.UsersService
	lrnsrv *learn.LearnService
	aisrv  *ai.AIService
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

	uiInstns := ui.NewUI()
	srv.ctxTimeout = time.Duration(ctxTimeout)
	srv.adminUUID = adminUUID
	srv.uiInstns = uiInstns
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
	aiTimeout := time.Duration(GetEnvInt("AI_TIMEOUT", 30))

	usrsrv, err := users.NewUS(srv.ctxTimeout, srv.adminUUID,
		srv.uiInstns, srv.sm, srv.b, srv.log)
	if err != nil {
		srv.log.Fatal("Failed to create users service", zap.Error(err))
	}
	srv.usrsrv = usrsrv

	lrnsrv, err := learn.NewLS(srv.sm, srv.ctxTimeout, srv.log)
	if err != nil {
		srv.log.Fatal("Failed to create learn service", zap.Error(err))
	}
	srv.lrnsrv = lrnsrv

	airsrv, err := ai.NewAIS(aiTimeout, srv.sm,
		srv.uiInstns, srv.b, srv.log)
	if err != nil {
		srv.log.Fatal("Failed to create ai service", zap.Error(err))
	}
	srv.aisrv = airsrv

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
			if err := srv.usrsrv.HandleRoutes(msg.Text, c); err != nil {
				srv.log.Error("Failed to handle state", zap.Error(err))
				return fmt.Errorf("%s: handle state: %w", op, err)
			}
		case "Learning":
			if err := srv.learningTask(c); err != nil {
				srv.log.Error("Failed to handle learning task", zap.Error(err))
				c.Send("Something went wrong. Try again later")
				return fmt.Errorf("%s: handle learning task: %w", op, err)
			}
		case "Shiritori":
			if err := srv.shiritori(c); err != nil {
				srv.log.Error("Failed to handle shiritori", zap.Error(err))
				c.Send("Something went wrong. Try again later")
				return fmt.Errorf("%s: handle shiritori: %w", op, err)
			}
		case "Chatting":
			if err := srv.chatting(c); err != nil {
				srv.log.Error("Failed to handle chatting", zap.Error(err))
				c.Send("Something went wrong. Try again later")
				return fmt.Errorf("%s: handle chatting: %w", op, err)
			}
		case "TTS":
			if err := srv.tts(c); err != nil {
				srv.log.Error("Failed to handle chatting", zap.Error(err))
				c.Send("Something went wrong. Try again later")
				return fmt.Errorf("%s: handle chatting: %w", op, err)
			}
		default:
			if err := srv.handleDefaultState(c); err != nil {
				srv.log.Error("Failed to handle default state", zap.Error(err))
				c.Send("Something went wrong. Try again later")
				return fmt.Errorf("%s: handle default state: %w", op, err)
			}
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

// learningTask handles 'Learning' command
func (srv *Server) learningTask(c tele.Context) error {
	const op = "router.learningTask"

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

	if err := c.Send(fmt.Sprintf("Task:\n%v", (*tasksPtr)[0])); err != nil {
		return fmt.Errorf("%s: send task: %w", op, err)
	}

	return nil
}

// shiritori handles 'Shiritori' command
func (srv *Server) shiritori(c tele.Context) error {
	const op = "router.shiritori"

	if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateShiritori, nil); err != nil {
		return fmt.Errorf("%s: set state: %w", op, err)
	}
	return c.Send(
		"Shiritori mode activated."+
			"To exit push '/stop' button. \nWrite any word.",
		srv.uiInstns.Stopmenu)
}

// chatting handles 'Chatting' command
func (srv *Server) chatting(c tele.Context) error {
	const op = "router.chatting"

	userUUID := c.Sender().ID
	reqtrace := uuid.NewString()
	ud, err := srv.usrsrv.GetData(userUUID, reqtrace)
	if err != nil {
		return fmt.Errorf("%s: get user data: %w", op, err)
	}

	srv.log.Debug("Chatting mode activated",
		zap.Int64("user_id", c.Sender().ID),
		zap.String("reqtrace", reqtrace))

	chattingSession := sm.ChattingSession{
		SystemPrompt: ud.SystemPrompt,
	}
	jsonData, err := json.Marshal(chattingSession)
	if err != nil {
		return fmt.Errorf("%s: marshal chatting session: %w", op, err)
	}

	if err := srv.sm.SetUserCtx(userUUID, sm.StateChatting, jsonData); err != nil {
		return fmt.Errorf("%s: set state: %w", op, err)
	}

	if err := c.Send(
		"Chatting mode activated."+
			"To exit write '/stop'.",
		srv.uiInstns.Stopmenu); err != nil {
		return fmt.Errorf("%s: send message: %w", op, err)
	}

	return nil
}

// tts activates TTS mode.
func (srv *Server) tts(c tele.Context) error {
	const op = "router.tts"

	if err := c.Send("TTS mode activated."); err != nil {
		return fmt.Errorf("%s: send message: %w", op, err)
	}

	if err := srv.sm.SetUserCtx(c.Sender().ID, sm.StateTTS, nil); err != nil {
		return fmt.Errorf("%s: set state: %w", op, err)
	}

	return nil
}

// handleDefaultState handles any other message
func (srv *Server) handleDefaultState(c tele.Context) error {
	const op = "router.handleDefaultState"

	usrctx, err := srv.sm.GetUserCtx(c.Sender().ID)
	if err != nil {
		return fmt.Errorf("%s: get user state: %w", op, err)
	}
	if c.Sender().ID == srv.adminUUID && usrctx.State != sm.StateAdminNotCommand {
		if err := srv.handleAdmin(c); err != nil {
			return fmt.Errorf("%s: handle admin: %w", op, err)
		}
		return nil
	}
	if err := srv.handleState(c); err != nil {
		return fmt.Errorf("%s: handle state: %w", op, err)
	}
	return nil
}
