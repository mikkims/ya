package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func TestFilePersistsAndRestoresURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "urls.json")
	storage, err := NewFile(path, zerolog.Nop())
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	if err := storage.Save(context.Background(), "4rSPg8ap", "http://yandex.ru"); err != nil {
		t.Fatalf("save first URL: %v", err)
	}
	if err := storage.Save(context.Background(), "edVPg3ks", "http://ya.ru"); err != nil {
		t.Fatalf("save second URL: %v", err)
	}

	restored, err := NewFile(path, zerolog.Nop())
	if err != nil {
		t.Fatalf("restore storage: %v", err)
	}
	for id, want := range map[string]string{
		"4rSPg8ap": "http://yandex.ru",
		"edVPg3ks": "http://ya.ru",
	} {
		got, ok, err := restored.Get(context.Background(), id)
		if err != nil || !ok || got != want {
			t.Errorf("Get(%q) = %q, %v; want %q, true", id, got, ok, want)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read storage file: %v", err)
	}
	var records []fileRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("decode storage file: %v", err)
	}
	if len(records) != 2 || records[0].UUID != "1" || records[1].UUID != "2" {
		t.Errorf("unexpected records: %+v", records)
	}
}

func TestFileRejectsDuplicateID(t *testing.T) {
	storage, err := NewFile(filepath.Join(t.TempDir(), "urls.json"), zerolog.Nop())
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	if err := storage.Save(context.Background(), "duplicate", "http://first.example"); err != nil {
		t.Fatalf("save URL: %v", err)
	}
	if err := storage.Save(context.Background(), "duplicate", "http://second.example"); err != ErrIDExists {
		t.Errorf("Save duplicate error = %v, want %v", err, ErrIDExists)
	}
}

func TestFileSaveBatchPersistsOnceAndIsAtomic(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "urls.json")
	store, err := NewFile(path, zerolog.Nop())
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	if _, err := store.SaveBatch(ctx, []URL{
		{ID: "first", OriginalURL: "https://first.example"},
		{ID: "second", OriginalURL: "https://second.example"},
	}); err != nil {
		t.Fatalf("SaveBatch() error = %v", err)
	}

	restored, err := NewFile(path, zerolog.Nop())
	if err != nil {
		t.Fatalf("restore storage: %v", err)
	}
	for _, id := range []string{"first", "second"} {
		if _, ok, err := restored.Get(ctx, id); err != nil || !ok {
			t.Fatalf("Get(%q): ok=%v, err=%v", id, ok, err)
		}
	}

	_, err = store.SaveBatch(ctx, []URL{
		{ID: "third", OriginalURL: "https://third.example"},
		{ID: "first", OriginalURL: "https://duplicate.example"},
	})
	if !errors.Is(err, ErrIDExists) {
		t.Fatalf("SaveBatch() error = %v, want %v", err, ErrIDExists)
	}
	if _, ok, err := store.Get(ctx, "third"); err != nil || ok {
		t.Fatalf("partially saved URL: ok=%v, err=%v", ok, err)
	}
}

func TestFileRejectsDuplicateOriginalURLAfterReload(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "urls.json")
	store, err := NewFile(path, zerolog.Nop())
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	if err := store.Save(ctx, "existing-id", "https://example.com"); err != nil {
		t.Fatalf("prepare storage: %v", err)
	}

	restored, err := NewFile(path, zerolog.Nop())
	if err != nil {
		t.Fatalf("reload storage: %v", err)
	}
	err = restored.Save(ctx, "new-id", "https://example.com")
	var conflict *OriginalURLExistsError
	if !errors.As(err, &conflict) || conflict.ID != "existing-id" {
		t.Fatalf("Save() error = %#v, want existing ID", err)
	}
}

func TestNewFileRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "urls.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("write invalid storage: %v", err)
	}
	if _, err := NewFile(path, zerolog.Nop()); err == nil {
		t.Fatal("NewFile() error = nil, want an error")
	}
}

func TestNewFileRejectsDuplicateUUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "urls.json")
	data := []byte(`[
  {"uuid":"1","short_url":"first","original_url":"http://first.example"},
  {"uuid":"1","short_url":"second","original_url":"http://second.example"}
]`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write storage: %v", err)
	}
	if _, err := NewFile(path, zerolog.Nop()); err == nil {
		t.Fatal("NewFile() error = nil, want an error")
	}
}
