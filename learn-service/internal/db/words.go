// Package db words.go contains operation with words psql table.
package db

import (
	"context"
	"fmt"
	"strconv"

	"learn/internal/parser"

	sq "github.com/Masterminds/squirrel"
)

// NewWordsBulk insert many words to database.
func (d *DB) NewWordsBulk(ctx context.Context, words []parser.Word) (int32, error) {
	const op = "db.NewWordsBulk"

	if len(words) == 0 {
		return 0, fmt.Errorf("%s: empty words", op)
	}

	insertBuilder := d.bd.Insert("words").
		Columns("word", "explain", "level", "first_letter")

	for _, word := range words {
		insertBuilder = insertBuilder.
			Values(word.Word, word.Explain, word.Level, word.FirstLetter)
	}

	query, args, err := insertBuilder.ToSql()
	if err != nil {
		return 0, fmt.Errorf("%s: create insert bulk query: %w", op, err)
	}

	res, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("%s: exec insert bulk query: %w", op, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: get rows affected: %w", op, err)
	}

	return int32(rowsAffected), nil
}

// GetWords update 'words' slice with words by level and serial.
func (d *DB) GetWords(ctx context.Context, searchData string, words *[]parser.Word) error {
	const op = "db.GetWords"

	query := d.bd.Select("word, explain, level, first_letter, serial").
		From("words")

	serial, err := strconv.Atoi(searchData)
	if err != nil {
		query = query.Where(sq.Eq{"word": searchData})
	} else {
		query = query.Where(sq.Eq{"serial": serial})
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: create get words query: %w", op, err)
	}

	if err := d.db.SelectContext(ctx, words, sql, args...); err != nil {
		return fmt.Errorf("%s: exec get words query: %w", op, err)
	}

	return nil
}

// DelWords delete by word and serial.
func (d *DB) DelWords(ctx context.Context, level string, serial int32) error {
	const op = "db.DelWords"

	query, args, err := d.bd.Delete("words").
		Where(sq.Eq{"word": level}).
		Where(sq.Eq{"serial": serial}).
		ToSql()
	if err != nil {
		return fmt.Errorf("%s: create del words query: %w", op, err)
	}

	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("%s: exec del words query: %w", op, err)
	}

	return nil
}
