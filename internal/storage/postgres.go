package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
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
	if errors.As(err, &pqErr) && pqErr.Code == pqerror.UniqueViolation {
		switch pqErr.Constraint {
		case shortURLPrimaryKeyConstraint:
			return ErrIDExists
		case originalURLUniqueConstraint:
			return s.originalURLExistsError(ctx, originalURL)
		}
	}

	return fmt.Errorf("save URL: %w", err)
}

func (s *PostgreSQL) SaveBatch(ctx context.Context, urls []URL) ([]URL, error) {
	if len(urls) == 0 {
		return []URL{}, nil
	}

	unique := make([]URL, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, url := range urls {
		if _, exists := seen[url.OriginalURL]; exists {
			continue
		}
		seen[url.OriginalURL] = struct{}{}
		unique = append(unique, url)
	}

	var query strings.Builder
	query.WriteString("INSERT INTO urls (short_url, original_url, user_id) VALUES ")
	args := make([]any, 0, len(unique)*3)
	for i, url := range unique {
		if i > 0 {
			query.WriteString(", ")
		}
		base := i*3 + 1
		_, _ = fmt.Fprintf(&query, "($%d, $%d, $%d)", base, base+1, base+2)
		args = append(args, url.ID, url.OriginalURL, url.UserID)
	}
	query.WriteString(" ON CONFLICT (original_url) DO UPDATE SET original_url = EXCLUDED.original_url")
	query.WriteString(" RETURNING short_url, original_url")

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqerror.UniqueViolation &&
			pqErr.Constraint == shortURLPrimaryKeyConstraint {
			return nil, ErrIDExists
		}
		return nil, fmt.Errorf("save URL batch: %w", err)
	}
	defer func() { _ = rows.Close() }()

	idsByOriginalURL := make(map[string]string, len(unique))
	for rows.Next() {
		var id, originalURL string
		if err := rows.Scan(&id, &originalURL); err != nil {
			return nil, fmt.Errorf("scan saved URL batch: %w", err)
		}
		idsByOriginalURL[originalURL] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved URL batch: %w", err)
	}

	result := make([]URL, len(urls))
	for i, url := range urls {
		id, ok := idsByOriginalURL[url.OriginalURL]
		if !ok {
			return nil, fmt.Errorf("save URL batch: no result for %q", url.OriginalURL)
		}
		url.ID = id
		result[i] = url
	}
	return result, nil
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
	var deleted bool
	err := s.db.QueryRowContext(ctx,
		"SELECT original_url, is_deleted FROM urls WHERE short_url = $1",
		id,
	).Scan(&originalURL, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get URL: %w", err)
	}
	if deleted {
		return "", false, ErrURLDeleted
	}

	return originalURL, true, nil
}

func (s *PostgreSQL) GetByUser(ctx context.Context, userID string) ([]URL, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT short_url, original_url FROM urls WHERE user_id = $1 AND NOT is_deleted ORDER BY created_at",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user URLs: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (s *PostgreSQL) Delete(ctx context.Context, ids []string, userID string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE urls SET is_deleted = TRUE WHERE user_id = $1 AND short_url = ANY($2)",
		userID,
		pq.Array(ids),
	)
	if err != nil {
		return fmt.Errorf("delete user URLs: %w", err)
	}
	return nil
}
