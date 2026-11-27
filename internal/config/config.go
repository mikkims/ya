package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

const (
	// DefaultFileStoragePath путь к файлу хранилища по умолчанию.
	DefaultFileStoragePath = ""
	// DefaultDeleteBufferSize задаёт порог числа URL для сброса удалений по умолчанию.
	DefaultDeleteBufferSize = 100
	// DefaultDeleteFlushInterval интервал сброса удалений по умолчанию.
	DefaultDeleteFlushInterval = time.Second
)

// Config содержит параметры приложения.
type Config struct {
	// AuditFile путь к файлу аудита.
	AuditFile string
	// AuditURL адрес HTTP-получателя аудита.
	AuditURL string
	// ServerAddress адрес HTTP-сервера.
	ServerAddress string
	// BaseURL базовый адрес коротких URL.
	BaseURL string
	// FileStoragePath путь к файлу хранилища.
	FileStoragePath string
	// DatabaseDSN строка подключения к базе данных.
	DatabaseDSN string
	// DeleteBufferSize задаёт порог числа URL для сброса накопленных удалений.
	DeleteBufferSize int
	// DeleteFlushInterval интервал сброса очереди удаления.
	DeleteFlushInterval time.Duration
}

// Load разбирает флаги и применяет значения окружения с приоритетом над флагами.
// Пустые значения окружения и некорректные или неположительные числовые параметры
// не заменяют значения флагов. Функция использует глобальный набор flag.CommandLine.
func Load() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "audit log file")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "audit HTTP receiver")

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base address of the shortened URL")
	flag.StringVar(&cfg.FileStoragePath, "f", DefaultFileStoragePath, "path to the URL storage file")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "PostgreSQL connection string")
	flag.IntVar(&cfg.DeleteBufferSize, "delete-buffer-size", DefaultDeleteBufferSize, "maximum number of URLs in a deletion batch")
	flag.DurationVar(&cfg.DeleteFlushInterval, "delete-flush-interval", DefaultDeleteFlushInterval, "maximum delay before flushing URL deletions")
	flag.Parse()
	cfg.AuditFile = getEnv("AUDIT_FILE", cfg.AuditFile)
	cfg.AuditURL = getEnv("AUDIT_URL", cfg.AuditURL)

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
