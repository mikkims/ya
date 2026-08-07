package config

import "flag"

type Config struct {
	ServerAddress string
	BaseURL       string
}

func Parse() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base address of the shortened URL")
	flag.Parse()

	return cfg
}
