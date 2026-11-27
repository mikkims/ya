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

func TestBuildAudit(t *testing.T) {
	for _, tt := range []struct {
		name, file, url   string
		wantFile, wantErr bool
	}{
		{name: "disabled"},
		{name: "file", file: "audit.log", wantFile: true},
		{name: "http", url: "http://localhost:1234"},
		{name: "both", file: "audit.log", url: "http://localhost:1234", wantFile: true},
		{name: "invalid URL", file: "audit.log", url: "invalid", wantErr: true},
		{name: "invalid file", file: "missing/audit.log", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := ""
			if tt.file != "" {
				path = filepath.Join(t.TempDir(), tt.file)
			}
			publisher, file, err := buildAudit(&config.Config{AuditFile: path, AuditURL: tt.url})
			if (err != nil) != tt.wantErr || (file != nil) != tt.wantFile {
				t.Fatalf("publisher=%v file=%v err=%v", publisher, file, err)
			}
			if file != nil {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
