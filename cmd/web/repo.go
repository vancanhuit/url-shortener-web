package main

import (
	"database/sql"
	"errors"
)

var ErrRecordNotFound = errors.New("record not found")

type Repo struct {
	DB *sql.DB
}

func (r *Repo) Insert(url, alias string) error {
	stmt := `
		INSERT INTO urls (original_url, alias) VALUES ($1, $2)
		ON CONFLICT (original_url)
		DO NOTHING;`
	_, err := r.DB.Exec(stmt, url, alias)
	return err
}

func (r *Repo) GetOriginalURL(alias string) (string, error) {
	var originalURL string
	stmt := `SELECT original_url FROM urls WHERE alias = $1;`
	err := r.DB.QueryRow(stmt, alias).Scan(&originalURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrRecordNotFound
		}
		return "", err
	}
	return originalURL, nil
}
