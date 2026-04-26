package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrRecordNotFound = errors.New("record not found")

type Repo struct {
	DB *sql.DB
}

func (r *Repo) Insert(ctx context.Context, url, alias string) (result string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			err = fmt.Errorf("rollback tx: %w", rbErr)
		}
	}()
	var existingAlias string
	stmt := `WITH res AS (
		INSERT INTO urls (original_url, alias) VALUES ($1, $2)
		ON CONFLICT (original_url)
		DO NOTHING
		RETURNING alias
	)
	SELECT alias FROM res
	UNION ALL
	SELECT alias FROM urls WHERE original_url = $1;`

	err = tx.QueryRowContext(ctx, stmt, url, alias).Scan(&existingAlias)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("query url: %w", err)
	}
	if existingAlias != "" {
		alias = existingAlias
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}
	return alias, nil
}

func (r *Repo) GetOriginalURL(ctx context.Context, alias string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var originalURL string
	stmt := `SELECT original_url FROM urls WHERE alias = $1;`

	err := r.DB.QueryRowContext(ctx, stmt, alias).Scan(&originalURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrRecordNotFound
		}
		return "", fmt.Errorf("query original url: %w", err)
	}
	return originalURL, nil
}
