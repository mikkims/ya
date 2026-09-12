package storage

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

const (
	insertURLQuery = "INSERT INTO urls (short_url, original_url) VALUES ($1, $2)"
	selectURLQuery = "SELECT original_url FROM urls WHERE short_url = $1"
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
		WillReturnError(&pq.Error{Code: "23505"})

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

func TestPostgreSQLSaveBatchCommits(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("first", "https://first.example").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("second", "https://second.example").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := store.SaveBatch(context.Background(), []URL{
		{ID: "first", OriginalURL: "https://first.example"},
		{ID: "second", OriginalURL: "https://second.example"},
	})
	if err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}
}

func TestPostgreSQLSaveBatchRollsBack(t *testing.T) {
	tests := []struct {
		name    string
		dbErr   error
		wantErr error
	}{
		{name: "duplicate", dbErr: &pq.Error{Code: "23505"}, wantErr: ErrIDExists},
		{name: "database error", dbErr: errors.New("database unavailable"), wantErr: errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, mock := newPostgreSQLMock(t)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
				WithArgs("first", "https://first.example").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
				WithArgs("second", "https://second.example").
				WillReturnError(tt.dbErr)
			mock.ExpectRollback()

			err := store.SaveBatch(context.Background(), []URL{
				{ID: "first", OriginalURL: "https://first.example"},
				{ID: "second", OriginalURL: "https://second.example"},
			})
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

func TestPostgreSQLSaveBatchBeginError(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	wantErr := errors.New("begin unavailable")
	mock.ExpectBegin().WillReturnError(wantErr)

	err := store.SaveBatch(context.Background(), []URL{{ID: "first", OriginalURL: "https://first.example"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("SaveBatch() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestPostgreSQLSaveBatchCommitError(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	wantErr := errors.New("commit unavailable")
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertURLQuery)).
		WithArgs("first", "https://first.example").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(wantErr)

	err := store.SaveBatch(context.Background(), []URL{{ID: "first", OriginalURL: "https://first.example"}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("SaveBatch() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestPostgreSQLGet(t *testing.T) {
	store, mock := newPostgreSQLMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(selectURLQuery)).
		WithArgs("short-id").
		WillReturnRows(sqlmock.NewRows([]string{"original_url"}).AddRow("https://example.com"))

	got, ok, err := store.Get(context.Background(), "short-id")
	if err != nil || !ok || got != "https://example.com" {
		t.Fatalf("Get() = %q, %v, %v; want URL, true, nil", got, ok, err)
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
