package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

type PostgreSQL struct {
	db *sql.DB
}

func NewPostgreSQL(db *sql.DB) *PostgreSQL {
	return &PostgreSQL{db: db}
}

func (s *PostgreSQL) Save(ctx context.Context, id, originalURL string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO urls (short_url, original_url) VALUES ($1, $2)",
		id,
		originalURL,
	)
	if err == nil {
		return nil
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrIDExists
	}

	return fmt.Errorf("save URL: %w", err)
}

func (s *PostgreSQL) Get(ctx context.Context, id string) (string, bool, error) {
	var originalURL string
	err := s.db.QueryRowContext(ctx,
		"SELECT original_url FROM urls WHERE short_url = $1",
		id,
	).Scan(&originalURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get URL: %w", err)
	}

	return originalURL, true, nil
}
