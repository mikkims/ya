package main

import (
	"net/http"
	"os"

	"github.com/mikkims/ya/internal/config"
	appgzip "github.com/mikkims/ya/internal/gzip"
	"github.com/mikkims/ya/internal/handler"
	"github.com/mikkims/ya/internal/logger"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.Load()
	appLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	urlStorage, err := storage.NewFile(cfg.FileStoragePath)
	if err != nil {
		appLogger.Info().Err(err).Str("path", cfg.FileStoragePath).Msg("failed to initialize storage")
		return
	}
	shortenerService := service.NewShortener(urlStorage)
	router := handler.NewRouter(cfg.BaseURL, shortenerService)
	compressedRouter := appgzip.MiddlewareGzip(router)

	err = http.ListenAndServe(cfg.ServerAddress, logger.Middleware(appLogger)(compressedRouter))
	if err != nil {
		appLogger.Info().Err(err).Msg("server stopped")
	}
}
