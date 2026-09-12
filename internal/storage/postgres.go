package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

const (
	shortURLPrimaryKeyConstraint = "urls_pkey"
	originalURLUniqueConstraint  = "idx_urls_original_url_unique"
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
		switch pqErr.Constraint {
		case shortURLPrimaryKeyConstraint:
			return ErrIDExists
		case originalURLUniqueConstraint:
			return s.originalURLExistsError(ctx, originalURL)
		}
	}

	return fmt.Errorf("save URL: %w", err)
}

func (s *PostgreSQL) SaveBatch(ctx context.Context, urls []URL) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin URL batch transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback URL batch transaction: %w", rollbackErr)
		}
	}()

	for _, url := range urls {
		if _, err = tx.ExecContext(ctx,
			"INSERT INTO urls (short_url, original_url) VALUES ($1, $2)",
			url.ID,
			url.OriginalURL,
		); err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				switch pqErr.Constraint {
				case shortURLPrimaryKeyConstraint:
					return ErrIDExists
				case originalURLUniqueConstraint:
					return &OriginalURLExistsError{}
				}
			}
			return fmt.Errorf("save URL batch: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit URL batch: %w", err)
	}
	return nil
}

func (s *PostgreSQL) originalURLExistsError(ctx context.Context, originalURL string) error {
	var id string
	if err := s.db.QueryRowContext(ctx,
		"SELECT short_url FROM urls WHERE original_url = $1",
		originalURL,
	).Scan(&id); err != nil {
		return fmt.Errorf("get existing short URL: %w", err)
	}
	return &OriginalURLExistsError{ID: id}
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
