package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mikkims/ya/internal/config"
	"github.com/mikkims/ya/internal/storage"
	"github.com/rs/zerolog"
)

func TestBuildStorageFallback(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		wantFile bool
	}{
		{name: "memory"},
		{name: "file", filePath: filepath.Join(t.TempDir(), "urls.json"), wantFile: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, db, err := buildStorage(context.Background(), &config.Config{FileStoragePath: tt.filePath}, zerolog.Nop())
			if err != nil {
				t.Fatalf("buildStorage() error = %v", err)
			}
			if db != nil {
				t.Fatal("buildStorage() database must be nil without DSN")
			}

			_, isFile := got.(*storage.File)
			if isFile != tt.wantFile {
				t.Errorf("buildStorage() file storage = %v, want %v", isFile, tt.wantFile)
			}
		})
	}
}
