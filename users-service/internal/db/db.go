// Package db provides database access for users service grpc methods.
package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

type DB struct {
	db  *sqlx.DB
	log *zap.Logger
	bd  sq.StatementBuilderType
}

type User struct {
	UUID      int64  `db:"uuid" json:"uuid"`
	Level     string `db:"level" json:"level"`
	TaskID    int32  `db:"task_id" json:"task_id"`
	BestTask  int32  `db:"best_task" json:"best_task"`
	WorstTask int32  `db:"worst_task" json:"worst_task"`
	Streak    int32  `db:"streak" json:"streak"`
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

// NewDB creates new database connection.
func NewDB(log *zap.Logger) (*DB, error) {
	const op = "db.New"

	db, err := sqlx.Connect("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return nil, fmt.Errorf("%s: sqlx connect: %w", op, err)
	}

	db.SetMaxOpenConns(GetEnvInt("MAX_OPEN_CONNS", 15))
	db.SetMaxIdleConns(GetEnvInt("MAX_IDLE_CONNS", 10))
	db.SetConnMaxLifetime(time.Duration(GetEnvInt("MAX_LIFETIME", 15)) * time.Minute)
	db.SetConnMaxIdleTime(time.Duration(GetEnvInt("MAX_IDLETIME", 10)) * time.Minute)

	log.Debug("DB users succesfully connected")

	return &DB{
		db:  db,
		log: log,
		bd:  sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

// RegUser add user to database.
func (d *DB) RegUser(uuid int64, ctx context.Context, reqTrace string) error {
	const op = "db.RegUser"

	query, args, err := d.bd.Insert("users").
		Columns("uuid").
		Values(uuid).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: build insert query: %w", op, err)
	}

	d.log.Debug("RegUser query",
		zap.String("query", query),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: insert user: %w", op, err)
	}

	d.log.Debug("User succesfully registered",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return nil
}

// GetUser get user from database.
// Returns all user fields if user exists
func (d *DB) GetUser(uuid int64, ctx context.Context, reqTrace string) (*User, error) {
	const op = "db.GetUser"

	query, args, err := d.bd.Select("level", "task_id", "best_task", "worst_task", "streak").
		From("users").
		Where(sq.Eq{"uuid": uuid}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build get query: %w", op, err)
	}

	d.log.Debug("GetUser request",
		zap.String("query", query),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	d.log.Debug("GetUser query", zap.String("query", query))

	var user User
	if err := d.db.GetContext(ctx, &user, query, args...); err != nil {
		return nil, fmt.Errorf("%s: get user: %w", op, err)
	}

	d.log.Debug("succesfully get user",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return &user, nil
}

func (d *DB) UpdUser(user User, ctx context.Context, reqTrace string) error {
	const op = "db.UpdUser"

	query, args, err := d.bd.Update("users").
		SetMap(map[string]any{
			"level":      user.Level,
			"task_id":    user.TaskID,
			"best_task":  user.BestTask,
			"worst_task": user.WorstTask,
			"streak":     user.Streak,
		}).
		Where(sq.Eq{"uuid": user.UUID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: build update query: %w", op, err)
	}

	d.log.Debug("UpdUser query",
		zap.String("query", query),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: update user: %w", op, err)
	}

	d.log.Debug("User succesfully updated",
		zap.Int64("uuid", user.UUID),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return nil
}

// DelUser delete user from database
func (d *DB) DelUser(uuid int64, ctx context.Context, reqTrace string) error {
	const op = "db.DelUser"

	query, args, err := d.bd.Delete("users").
		Where(sq.Eq{"uuid": uuid}).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: build delete query: %w", op, err)
	}

	d.log.Debug("DelUser query",
		zap.String("query", query),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: delete user: %w", op, err)
	}

	d.log.Debug("User succesfully deleted",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return nil
}

// UpdateStreak atomically updates the streak of a user
func (d *DB) UpdateStreak(uuid int64, ctx context.Context, reqTrace string, correct bool) error {
	const op = "db.UpdateStreak"

	currentDay := time.Now().UTC().Unix() / 86400

	caseExpr := sq.Expr(`CASE
		WHEN last_done_day = ? THEN streak
		WHEN last_done_day = ? THEN streak + 1
		ELSE 1
	END`, currentDay, currentDay-1)

	query, args, err := d.bd.Update("users").
		Set("streak", caseExpr).
		Set("last_done_day", currentDay).
		Where(sq.Eq{"uuid": uuid}).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: build update query: %w", op, err)
	}

	d.log.Debug("UpdateStreak query",
		zap.String("query", query),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: update streak: %w", op, err)
	}

	d.log.Debug("Streak succesfully updated",
		zap.Int64("uuid", uuid),
		zap.String("request_trace", reqTrace),
		zap.String("op", op))

	return nil
}
