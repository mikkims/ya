package storage

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mikkims/ya/internal/auth"
)

func TestMemorySaveBatchIsAtomic(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	if err := store.Save(ctx, "existing", "https://existing.example"); err != nil {
		t.Fatalf("prepare storage: %v", err)
	}

	_, err := store.SaveBatch(ctx, []URL{
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
			if _, err := store.SaveBatch(ctx, []URL{{ID: id, OriginalURL: "https://example.com/" + id}}); err != nil {
				t.Errorf("SaveBatch() error = %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestMemorySaveBatchReturnsExistingID(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	if err := store.Save(ctx, "existing-id", "https://example.com"); err != nil {
		t.Fatalf("prepare storage: %v", err)
	}

	got, err := store.SaveBatch(ctx, []URL{{ID: "new-id", OriginalURL: "https://example.com"}})
	if err != nil || len(got) != 1 || got[0].ID != "existing-id" {
		t.Fatalf("SaveBatch() = %#v, %v; want existing-id", got, err)
	}
}

func TestMemoryRejectsDuplicateOriginalURL(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	if err := store.Save(ctx, "existing-id", "https://example.com"); err != nil {
		t.Fatalf("prepare storage: %v", err)
	}

	err := store.Save(ctx, "new-id", "https://example.com")
	var conflict *OriginalURLExistsError
	if !errors.As(err, &conflict) || conflict.ID != "existing-id" {
		t.Fatalf("Save() error = %#v, want existing ID", err)
	}
}

func TestMemoryDeleteChecksOwner(t *testing.T) {
	store := NewMemory()
	ownerContext := auth.WithUserID(context.Background(), "owner")
	if err := store.Save(ownerContext, "short-id", "https://example.com"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Delete(context.Background(), []string{"short-id"}, "other"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok, err := store.Get(context.Background(), "short-id"); err != nil || !ok {
		t.Fatalf("another user deleted URL: ok=%v, err=%v", ok, err)
	}
	if err := store.Delete(context.Background(), []string{"short-id"}, "owner"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := store.Get(context.Background(), "short-id"); !errors.Is(err, ErrURLDeleted) {
		t.Fatalf("Get() error = %v, want %v", err, ErrURLDeleted)
	}
}
