package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestMemorySaveBatchIsAtomic(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	if err := store.Save(ctx, "existing", "https://existing.example"); err != nil {
		t.Fatalf("prepare storage: %v", err)
	}

	err := store.SaveBatch(ctx, []URL{
		{ID: "new", OriginalURL: "https://new.example"},
		{ID: "existing", OriginalURL: "https://duplicate.example"},
	})
	if !errors.Is(err, ErrIDExists) {
		t.Fatalf("SaveBatch() error = %v, want %v", err, ErrIDExists)
	}
	if _, ok, err := store.Get(ctx, "new"); err != nil || ok {
		t.Fatalf("partially saved URL: ok=%v, err=%v", ok, err)
	}
}

func TestMemorySaveBatchConcurrent(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := string(rune('a' + i))
			if err := store.SaveBatch(ctx, []URL{{ID: id, OriginalURL: "https://example.com/" + id}}); err != nil {
				t.Errorf("SaveBatch() error = %v", err)
			}
		}()
	}
	wg.Wait()
}
