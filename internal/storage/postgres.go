package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/mikkims/ya/internal/auth"
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
	userID := auth.UserID(ctx)
	var err error
	if userID == "" {
		_, err = s.db.ExecContext(ctx,
			"INSERT INTO urls (short_url, original_url) VALUES ($1, $2)",
			id,
			originalURL,
		)
	} else {
		_, err = s.db.ExecContext(ctx,
			"INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, $3)",
			id,
			originalURL,
			userID,
		)
	}
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
		if url.UserID == "" {
			_, err = tx.ExecContext(ctx,
				"INSERT INTO urls (short_url, original_url) VALUES ($1, $2)",
				url.ID,
				url.OriginalURL,
			)
		} else {
			_, err = tx.ExecContext(ctx,
				"INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, $3)",
				url.ID,
				url.OriginalURL,
				url.UserID,
			)
		}
		if err != nil {
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

func (s *PostgreSQL) GetByUser(ctx context.Context, userID string) ([]URL, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT short_url, original_url FROM urls WHERE user_id = $1 ORDER BY created_at",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user URLs: %w", err)
	}
	defer rows.Close()

	urls := make([]URL, 0)
	for rows.Next() {
		var url URL
		if err := rows.Scan(&url.ID, &url.OriginalURL); err != nil {
			return nil, fmt.Errorf("scan user URL: %w", err)
		}
		url.UserID = userID
		urls = append(urls, url)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user URLs: %w", err)
	}
	return urls, nil
}
