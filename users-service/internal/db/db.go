// Package db provides database access for users service grpc methods.
package db

import (
	"context"
	"fmt"
	"os"
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

// NewDB creates new database connection.
func NewDB(log *zap.Logger) (*DB, error) {
	const op = "db.New"

	db, err := sqlx.Connect("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return nil, fmt.Errorf("%s: sqlx connect: %w", op, err)
	}

	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	log.Info("DB users succesfully connected")

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
func (d *DB) RegUser(uuid int64, ctx context.Context) error {
	const op = "db.RegUser"

	query, args, err := d.bd.Insert("users").
		Columns("uuid").
		Values(uuid).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: build insert query: %w", op, err)
	}

	d.log.Info("RegUser query", zap.String("query", query))

	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: insert user: %w", op, err)
	}

	d.log.Info("User succesfully registered", zap.Int64("uuid", uuid))

	return nil
}

// GetUser get user from database.
// Returns all user fields if user exists
func (d *DB) GetUser(uuid int64, ctx context.Context) (*User, error) {
	const op = "db.GetUser"

	d.log.Info("GetUser", zap.Int64("uuid", uuid))

	query, args, err := d.bd.Select("level", "task_id", "best_task", "worst_task", "streak").
		From("users").
		Where(sq.Eq{"uuid": uuid}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build get query: %w", op, err)
	}

	d.log.Info("GetUser query", zap.String("query", query))

	var user User
	if err := d.db.GetContext(ctx, &user, query, args...); err != nil {
		return nil, fmt.Errorf("%s: get user: %w", op, err)
	}

	d.log.Info("User struct after scan", zap.Any("user", user))

	return &user, nil
}

// DelUser delete user from database
func (d *DB) DelUser(uuid string, ctx context.Context) error {
	const op = "db.DelUser"

	query, args, err := d.bd.Delete("users").
		Where(sq.Eq{"uuid": uuid}).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: build delete query: %w", op, err)
	}

	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: delete user: %w", op, err)
	}

	return nil
}
