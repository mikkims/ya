package migrations

import (
	"strings"
	"testing"
)

func TestOriginalURLUniqueMigrationIsEmbedded(t *testing.T) {
	up, err := files.ReadFile("000002_original_url_unique.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	if !strings.Contains(string(up), "CREATE UNIQUE INDEX idx_urls_original_url_unique ON urls (original_url)") {
		t.Fatalf("unexpected up migration: %s", up)
	}

	down, err := files.ReadFile("000002_original_url_unique.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	if !strings.Contains(string(down), "DROP INDEX IF EXISTS idx_urls_original_url_unique") {
		t.Fatalf("unexpected down migration: %s", down)
	}
}
