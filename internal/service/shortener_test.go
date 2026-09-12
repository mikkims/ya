package service

import (
	"context"
	"errors"
	"testing"
)

type storageStub struct {
	saveErr error
	getURL  string
	getOK   bool
	getErr  error
}

func (s *storageStub) Save(_ context.Context, _, _ string) error {
	return s.saveErr
}

func (s *storageStub) Get(_ context.Context, _ string) (string, bool, error) {
	return s.getURL, s.getOK, s.getErr
}

func TestShortenerGetPropagatesStorageError(t *testing.T) {
	wantErr := errors.New("storage unavailable")
	shortener := NewShortener(&storageStub{getErr: wantErr})

	_, _, err := shortener.Get(context.Background(), "id")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Get() error = %v, want %v", err, wantErr)
	}
}
