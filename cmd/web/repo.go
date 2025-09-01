package main

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrRecordNotFound = errors.New("record not found")

type Repo struct {
	DB *sql.DB
}

func (r *Repo) Insert(url, alias string) (string, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = tx.QueryRowContext(ctx, stmt, url, alias).Scan(&existingAlias)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if existingAlias != "" {
		alias = existingAlias
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return alias, nil
}

func (r *Repo) GetOriginalURL(alias string) (string, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var originalURL string
	stmt := `SELECT original_url FROM urls WHERE alias = $1;`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = tx.QueryRowContext(ctx, stmt, alias).Scan(&originalURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrRecordNotFound
		}
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return originalURL, nil
}
