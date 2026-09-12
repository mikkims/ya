package config

import (
	"flag"
	"os"
	"testing"
)

func TestFileStoragePathPriority(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envValue string
		want     string
	}{
		{name: "default", want: ""},
		{name: "flag", args: []string{"-f", "flag-storage.json"}, want: "flag-storage.json"},
		{name: "environment", args: []string{"-f", "flag-storage.json"}, envValue: "env-storage.json", want: "env-storage.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalCommandLine := flag.CommandLine
			originalArgs := os.Args
			defer func() {
				flag.CommandLine = originalCommandLine
				os.Args = originalArgs
			}()

			flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
			os.Args = append([]string{"shortener"}, tt.args...)
			t.Setenv("FILE_STORAGE_PATH", tt.envValue)

			cfg := Load()
			if cfg.FileStoragePath != tt.want {
				t.Errorf("FileStoragePath = %q, want %q", cfg.FileStoragePath, tt.want)
			}
		})
	}
}

func TestDatabaseDSNPriority(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envValue string
		want     string
	}{
		{name: "default"},
		{name: "flag", args: []string{"-d", "postgres://flag"}, want: "postgres://flag"},
		{name: "environment", args: []string{"-d", "postgres://flag"}, envValue: "postgres://environment", want: "postgres://environment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalCommandLine := flag.CommandLine
			originalArgs := os.Args
			defer func() {
				flag.CommandLine = originalCommandLine
				os.Args = originalArgs
			}()

			flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
			os.Args = append([]string{"shortener"}, tt.args...)
			t.Setenv("DATABASE_DSN", tt.envValue)

			cfg := Load()
			if cfg.DatabaseDSN != tt.want {
				t.Errorf("DatabaseDSN = %q, want %q", cfg.DatabaseDSN, tt.want)
			}
		})
	}
}
