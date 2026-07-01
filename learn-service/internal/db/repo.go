// Package db repo.go contains connection to database
// and database structs.
package db

import (
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
	const op = "db.tasks.NewTasksDB"

	db, err := sqlx.Connect("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return nil, fmt.Errorf("%s: sqlx connect: %w", op, err)
	}

	db.SetMaxOpenConns(GetEnvInt("MaxOpenConns", 15))
	db.SetMaxIdleConns(GetEnvInt("MaxIdleConns", 10))
	db.SetConnMaxLifetime(time.Duration(GetEnvInt("ConnMaxLifetime", 15)) * time.Minute)
	db.SetConnMaxIdleTime(time.Duration(GetEnvInt("ConnMaxIdleTime", 10)) * time.Minute)

	log.Debug("DB learn sucesfully connected")

	return &DB{
		db:  db,
		log: log,
		bd:  sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}
