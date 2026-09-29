package service

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mikkims/ya/internal/storage"
)

type deleteStorageStub struct {
	storageStub
	mu      sync.Mutex
	batches [][]string
	flushed chan struct{}
}

func (s *deleteStorageStub) Delete(_ context.Context, ids []string, _ string) error {
	s.mu.Lock()
	s.batches = append(s.batches, append([]string(nil), ids...))
	s.mu.Unlock()
	select {
	case s.flushed <- struct{}{}:
	default:
	}
	return nil
}

func TestDeleteWorkerFlushesFullBuffer(t *testing.T) {
	store := &deleteStorageStub{flushed: make(chan struct{}, 1)}
	shortener := NewShortener(store, DeleteOptions{BufferSize: 2, FlushInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- shortener.RunDeleteWorker(ctx) }()

	if err := shortener.Delete(ctx, []string{"one"}, "user"); err != nil {
		t.Fatalf("enqueue first deletion: %v", err)
	}
	if err := shortener.Delete(ctx, []string{"two"}, "user"); err != nil {
		t.Fatalf("enqueue second deletion: %v", err)
	}
	select {
	case <-store.flushed:
	case <-time.After(time.Second):
		t.Fatal("delete buffer was not flushed")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunDeleteWorker() error = %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.batches) != 1 || !slices.Equal(store.batches[0], []string{"one", "two"}) {
		t.Fatalf("batches = %v, want [[one two]]", store.batches)
	}
}

func TestDeleteWorkerFlushesOnShutdown(t *testing.T) {
	store := &deleteStorageStub{flushed: make(chan struct{}, 1)}
	shortener := NewShortener(store, DeleteOptions{BufferSize: 10, FlushInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- shortener.RunDeleteWorker(ctx) }()

	if err := shortener.Delete(ctx, []string{"one"}, "user"); err != nil {
		t.Fatalf("enqueue deletion: %v", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunDeleteWorker() error = %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.batches) != 1 || !slices.Equal(store.batches[0], []string{"one"}) {
		t.Fatalf("batches = %v, want [[one]]", store.batches)
	}
}

func TestDeleteWorkerFlushesOnInterval(t *testing.T) {
	store := &deleteStorageStub{flushed: make(chan struct{}, 1)}
	shortener := NewShortener(store, DeleteOptions{BufferSize: 10, FlushInterval: 10 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- shortener.RunDeleteWorker(ctx) }()

	if err := shortener.Delete(ctx, []string{"one"}, "user"); err != nil {
		t.Fatalf("enqueue deletion: %v", err)
	}
	select {
	case <-store.flushed:
	case <-time.After(time.Second):
		t.Fatal("delete buffer was not flushed on interval")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunDeleteWorker() error = %v", err)
	}
}

type storageStub struct {
	saveErrs      []error
	saveBatchErrs []error
	saveBatchURLs []storage.URL
	savedBatches  [][]storage.URL
	getURL        string
	getOK         bool
	getErr        error
}

func (s *storageStub) SaveBatch(_ context.Context, urls []storage.URL) ([]storage.URL, error) {
	s.savedBatches = append(s.savedBatches, append([]storage.URL(nil), urls...))
	if len(s.saveBatchErrs) == 0 {
		if s.saveBatchURLs != nil {
			return append([]storage.URL(nil), s.saveBatchURLs...), nil
		}
		return urls, nil
	}
	err := s.saveBatchErrs[0]
	s.saveBatchErrs = s.saveBatchErrs[1:]
	return nil, err
}

func TestShortenerSaveBatchUsesStorageIDs(t *testing.T) {
	store := &storageStub{saveBatchURLs: []storage.URL{{ID: "existing-id", OriginalURL: "https://example.com"}}}
	shortener := NewShortener(store)

	got, err := shortener.SaveBatch(context.Background(), []string{"https://example.com"})
	if err != nil || !slices.Equal(got, []string{"existing-id"}) {
		t.Fatalf("SaveBatch() = %v, %v; want existing-id", got, err)
	}
}

func TestShortenerSaveBatch(t *testing.T) {
	store := &storageStub{}
	shortener := NewShortener(store)
	generated := []string{"same-id", "same-id", "second-id"}
	shortener.generateID = func() string {
		id := generated[0]
		generated = generated[1:]
		return id
	}

	got, err := shortener.SaveBatch(context.Background(), []string{"https://first.example", "https://second.example"})
	if err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}
	if !slices.Equal(got, []string{"same-id", "second-id"}) {
		t.Fatalf("SaveBatch() = %v, want IDs in input order", got)
	}
	if len(store.savedBatches) != 1 || len(store.savedBatches[0]) != 2 {
		t.Fatalf("saved batches = %#v, want one batch with two URLs", store.savedBatches)
	}
}

func TestShortenerSaveBatchRetriesWholeBatch(t *testing.T) {
	store := &storageStub{saveBatchErrs: []error{storage.ErrIDExists, nil}}
	shortener := NewShortener(store)
	generated := []string{"first-a", "first-b", "second-a", "second-b"}
	shortener.generateID = func() string {
		id := generated[0]
		generated = generated[1:]
		return id
	}

	got, err := shortener.SaveBatch(context.Background(), []string{"https://first.example", "https://second.example"})
	if err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}
	if !slices.Equal(got, []string{"second-a", "second-b"}) || len(store.savedBatches) != 2 {
		t.Fatalf("SaveBatch() = %v, attempts = %d; want regenerated second batch", got, len(store.savedBatches))
	}
}

func TestShortenerSaveBatchErrors(t *testing.T) {
	storageErr := errors.New("unavailable")
	tests := []struct {
		name  string
		urls  []string
		store *storageStub
		want  error
	}{
		{name: "empty", want: ErrEmptyBatch},
		{name: "storage", urls: []string{"https://example.com"}, store: &storageStub{saveBatchErrs: []error{storageErr}},
			want: storageErr},
		{name: "collisions exhausted", urls: []string{"https://example.com"},
			store: &storageStub{saveBatchErrs: []error{storage.ErrIDExists, storage.ErrIDExists, storage.ErrIDExists}},
			want:  ErrSaveAttemptsExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tt.store
			if store == nil {
				store = &storageStub{}
			}
			shortener := NewShortener(store)
			_, err := shortener.SaveBatch(context.Background(), tt.urls)
			if !errors.Is(err, tt.want) {
				t.Fatalf("SaveBatch() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func (s *storageStub) Save(_ context.Context, _, _ string) error {
	if len(s.saveErrs) == 0 {
		return nil
	}
	err := s.saveErrs[0]
	s.saveErrs = s.saveErrs[1:]
	return err
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

func TestShortenerReturnsExistingIDForOriginalURL(t *testing.T) {
	shortener := NewShortener(&storageStub{saveErrs: []error{&storage.OriginalURLExistsError{ID: "existing-id"}}})

	id, err := shortener.Save(context.Background(), "https://example.com")
	if !errors.Is(err, ErrOriginalURLExists) || id != "existing-id" {
		t.Fatalf("Save() = %q, %v; want existing-id, %v", id, err, ErrOriginalURLExists)
	}
}

func TestShortenerRetriesShortIDConflict(t *testing.T) {
	shortener := NewShortener(&storageStub{saveErrs: []error{storage.ErrIDExists, nil}})
	generated := []string{"conflicting-id", "new-id"}
	shortener.generateID = func() string {
		id := generated[0]
		generated = generated[1:]
		return id
	}

	id, err := shortener.Save(context.Background(), "https://example.com")
	if err != nil || id != "new-id" {
		t.Fatalf("Save() = %q, %v; want new-id, nil", id, err)
	}
}
