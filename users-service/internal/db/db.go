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
	Level     string `db:"level"`
	TaskID    int32  `db:"task_id"`
	BestTask  int32  `db:"best_task"`
	WorstTask int32  `db:"worst_task"`
	Streak    int64  `db:"streak"`
}

func getEnvInt(key string, defaultVal int) int {
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

	db.SetMaxOpenConns(getEnvInt("MaxOpenConns", 15))
	db.SetMaxIdleConns(getEnvInt("MaxIdleConns", 10))
	db.SetConnMaxLifetime(time.Duration(getEnvInt("MaxLifetime", 15)) * time.Minute)
	db.SetConnMaxIdleTime(time.Duration(getEnvInt("MaxIdleTime", 10)) * time.Minute)

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
