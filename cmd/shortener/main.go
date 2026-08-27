package main

import (
	"net/http"
	"os"

	"github.com/mikkims/ya/internal/config"
	"github.com/mikkims/ya/internal/handler"
	"github.com/mikkims/ya/internal/logger"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.Load()
	urlStorage := storage.NewMemory()
	shortenerService := service.NewShortener(urlStorage)
	appLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	router := handler.NewRouter(cfg.BaseURL, shortenerService)

	err := http.ListenAndServe(cfg.ServerAddress, logger.Middleware(appLogger)(router))
	if err != nil {
		appLogger.Info().Err(err).Msg("server stopped")
	}
}
