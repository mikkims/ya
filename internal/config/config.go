package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

const (
	DefaultFileStoragePath     = ""
	DefaultDeleteBufferSize    = 100
	DefaultDeleteFlushInterval = time.Second
)

type Config struct {
	ServerAddress       string
	BaseURL             string
	FileStoragePath     string
	DatabaseDSN         string
	DeleteBufferSize    int
	DeleteFlushInterval time.Duration
}

func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base address of the shortened URL")
	flag.StringVar(&cfg.FileStoragePath, "f", DefaultFileStoragePath, "path to the URL storage file")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "PostgreSQL connection string")
	flag.IntVar(&cfg.DeleteBufferSize, "delete-buffer-size", DefaultDeleteBufferSize, "maximum number of URLs in a deletion batch")
	flag.DurationVar(&cfg.DeleteFlushInterval, "delete-flush-interval", DefaultDeleteFlushInterval, "maximum delay before flushing URL deletions")
	flag.Parse()

	cfg.ServerAddress = getEnv("SERVER_ADDRESS", cfg.ServerAddress)
	cfg.BaseURL = getEnv("BASE_URL", cfg.BaseURL)
	cfg.FileStoragePath = getEnv("FILE_STORAGE_PATH", cfg.FileStoragePath)
	cfg.DatabaseDSN = getEnv("DATABASE_DSN", cfg.DatabaseDSN)
	cfg.DeleteBufferSize = getEnvInt("DELETE_BUFFER_SIZE", cfg.DeleteBufferSize)
	cfg.DeleteFlushInterval = getEnvDuration("DELETE_FLUSH_INTERVAL", cfg.DeleteFlushInterval)

	return cfg
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
