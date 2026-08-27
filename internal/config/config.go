package config

import (
	"flag"
	"os"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base address of the shortened URL")
	flag.Parse()

	cfg.ServerAddress = getEnv("SERVER_ADDRESS", cfg.ServerAddress)
	cfg.BaseURL = getEnv("BASE_URL", cfg.BaseURL)

	return cfg
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
