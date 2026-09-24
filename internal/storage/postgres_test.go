package storage

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

const (
	insertURLQuery = "INSERT INTO urls (short_url, original_url) VALUES ($1, $2)"
	selectURLQuery = "SELECT original_url, is_deleted FROM urls WHERE short_url = $1"
)

func newPostgreSQLMock(t *testing.T) (*PostgreSQL, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("close SQL mock: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet SQL expectations: %v", err)
		}
	})

	return NewPostgreSQL(db), mock
}

func TestPostgreSQLSave(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("short-id", "https://example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.Save(context.Background(), "short-id", "https://example.com"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestPostgreSQLSaveDuplicate(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("short-id", "https://example.com").
		WillReturnError(&pq.Error{Code: pqerror.UniqueViolation, Constraint: shortURLPrimaryKeyConstraint})

	err := store.Save(context.Background(), "short-id", "https://example.com")
	if !errors.Is(err, ErrIDExists) {
		t.Fatalf("Save() error = %v, want %v", err, ErrIDExists)
	}
}

func TestPostgreSQLSaveError(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	wantErr := errors.New("database unavailable")
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("short-id", "https://example.com").
		WillReturnError(wantErr)

	err := store.Save(context.Background(), "short-id", "https://example.com")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Save() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestPostgreSQLSaveBatchUsesSingleQuery(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	query := "INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, $3), ($4, $5, $6) " +
		"ON CONFLICT (original_url) DO UPDATE SET original_url = EXCLUDED.original_url RETURNING short_url, original_url"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("first", "https://first.example", "user", "second", "https://second.example", "user").
		WillReturnRows(sqlmock.NewRows([]string{"short_url", "original_url"}).
			AddRow("first", "https://first.example").
			AddRow("second", "https://second.example"))

	got, err := store.SaveBatch(context.Background(), []URL{
		{ID: "first", OriginalURL: "https://first.example", UserID: "user"},
		{ID: "second", OriginalURL: "https://second.example", UserID: "user"},
	})
	if err != nil || len(got) != 2 || got[0].ID != "first" || got[1].ID != "second" {
		t.Fatalf("SaveBatch() = %#v, %v", got, err)
	}
}

func TestPostgreSQLSaveBatchErrors(t *testing.T) {
	tests := []struct {
		name    string
		dbErr   error
		wantErr error
	}{
		{name: "duplicate", dbErr: &pq.Error{Code: pqerror.UniqueViolation, Constraint: shortURLPrimaryKeyConstraint}, wantErr: ErrIDExists},
		{name: "database error", dbErr: errors.New("database unavailable"), wantErr: errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, mock := newPostgreSQLMock(t)
			query := "INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, $3) " +
				"ON CONFLICT (original_url) DO UPDATE SET original_url = EXCLUDED.original_url RETURNING short_url, original_url"
			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs("first", "https://first.example", "").
				WillReturnError(tt.dbErr)

			_, err := store.SaveBatch(context.Background(), []URL{{ID: "first", OriginalURL: "https://first.example"}})
			if tt.name == "database error" {
				if err == nil || err.Error() != "save URL batch: "+tt.wantErr.Error() {
					t.Fatalf("SaveBatch() error = %v, want wrapped %v", err, tt.wantErr)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SaveBatch() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestPostgreSQLSaveBatchReturnsExistingID(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	query := "INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, $3) " +
		"ON CONFLICT (original_url) DO UPDATE SET original_url = EXCLUDED.original_url RETURNING short_url, original_url"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs("new-id", "https://example.com", "user").
		WillReturnRows(sqlmock.NewRows([]string{"short_url", "original_url"}).AddRow("existing-id", "https://example.com"))

	got, err := store.SaveBatch(context.Background(), []URL{{ID: "new-id", OriginalURL: "https://example.com", UserID: "user"}})
	if err != nil || len(got) != 1 || got[0].ID != "existing-id" {
		t.Fatalf("SaveBatch() = %#v, %v; want existing-id", got, err)
	}
}

func TestPostgreSQLSaveOriginalURLExists(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("new-id", "https://example.com").
		WillReturnError(&pq.Error{Code: pqerror.UniqueViolation, Constraint: originalURLUniqueConstraint})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT short_url FROM urls WHERE original_url = $1")).
		WithArgs("https://example.com").
		WillReturnRows(sqlmock.NewRows([]string{"short_url"}).AddRow("existing-id"))

	err := store.Save(context.Background(), "new-id", "https://example.com")
	var conflict *OriginalURLExistsError
	if !errors.As(err, &conflict) || conflict.ID != "existing-id" {
		t.Fatalf("Save() error = %#v, want existing ID", err)
	}
}

func TestPostgreSQLSaveOriginalURLLookupError(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	wantErr := errors.New("lookup unavailable")
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("new-id", "https://example.com").
		WillReturnError(&pq.Error{Code: pqerror.UniqueViolation, Constraint: originalURLUniqueConstraint})
	mock.ExpectQuery(regexp.QuoteMeta("SELECT short_url FROM urls WHERE original_url = $1")).
		WithArgs("https://example.com").
		WillReturnError(wantErr)

	err := store.Save(context.Background(), "new-id", "https://example.com")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Save() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestPostgreSQLGet(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(selectURLQuery)).
		WithArgs("short-id").
		WillReturnRows(sqlmock.NewRows([]string{"original_url", "is_deleted"}).AddRow("https://example.com", false))

	got, ok, err := store.Get(context.Background(), "short-id")
	if err != nil || !ok || got != "https://example.com" {
		t.Fatalf("Get() = %q, %v, %v; want URL, true, nil", got, ok, err)
	}
}

func TestPostgreSQLGetDeleted(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(selectURLQuery)).
		WithArgs("short-id").
		WillReturnRows(sqlmock.NewRows([]string{"original_url", "is_deleted"}).AddRow("https://example.com", true))

	_, ok, err := store.Get(context.Background(), "short-id")
	if !errors.Is(err, ErrURLDeleted) || ok {
		t.Fatalf("Get() = ok %v, error %v; want false, %v", ok, err, ErrURLDeleted)
	}
}

func TestPostgreSQLGetMissing(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(selectURLQuery)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	got, ok, err := store.Get(context.Background(), "missing")
	if err != nil || ok || got != "" {
		t.Fatalf("Get() = %q, %v, %v; want empty, false, nil", got, ok, err)
	}
}

func TestPostgreSQLGetError(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	wantErr := errors.New("database unavailable")
	mock.ExpectQuery(regexp.QuoteMeta(selectURLQuery)).
		WithArgs("short-id").
		WillReturnError(wantErr)

	_, _, err := store.Get(context.Background(), "short-id")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Get() error = %v, want wrapped %v", err, wantErr)
	}
}
