package main

import (
	"database/sql"
	"net/http"
	"os"

	_ "github.com/lib/pq"
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
	urlStorage, err := storage.NewFile(cfg.FileStoragePath, appLogger)
	if err != nil {
		appLogger.Info().Err(err).Str("path", cfg.FileStoragePath).Msg("failed to initialize storage")
		return
	}
	shortenerService := service.NewShortener(urlStorage)
	var database *sql.DB
	if cfg.DatabaseDSN != "" {
		database, err = sql.Open("postgres", cfg.DatabaseDSN)
		if err != nil {
			appLogger.Error().Err(err).Msg("failed to initialize database")
			return
		}
		defer func() {
			if err := database.Close(); err != nil {
				appLogger.Error().Err(err).Msg("failed to close database")
			}
		}()
	}

	router := handler.NewRouter(cfg.BaseURL, shortenerService)
	if database != nil {
		router = handler.NewRouterWithDatabase(cfg.BaseURL, shortenerService, database)
	}
	compressedRouter := appgzip.MiddlewareGzip(appLogger)(router)

	err = http.ListenAndServe(cfg.ServerAddress, logger.Middleware(appLogger)(compressedRouter))
	if err != nil {
		appLogger.Info().Err(err).Msg("server stopped")
	}
}
