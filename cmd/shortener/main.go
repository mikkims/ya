package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/mikkims/ya/internal/config"
	appdatabase "github.com/mikkims/ya/internal/database"
	appgzip "github.com/mikkims/ya/internal/gzip"
	"github.com/mikkims/ya/internal/handler"
	"github.com/mikkims/ya/internal/logger"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
	"github.com/mikkims/ya/migrations"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.Load()
	appLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	urlStorage, database, err := buildStorage(startupContext, cfg, appLogger)
	if err != nil {
		appLogger.Error().Err(err).Msg("failed to initialize storage")
		return
	}
	shortenerService := service.NewShortener(urlStorage)
	if database != nil {
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

func buildStorage(ctx context.Context, cfg *config.Config, appLogger zerolog.Logger) (service.URLStorage, *sql.DB, error) {
	if cfg.DatabaseDSN != "" {
		db, err := appdatabase.Open(ctx, "postgres", cfg.DatabaseDSN)
		if err != nil {
			return nil, nil, err
		}
		if err := migrations.Up(db); err != nil {
			_ = db.Close()
			return nil, nil, err
		}
		return storage.NewPostgreSQL(db), db, nil
	}

	if cfg.FileStoragePath != "" {
		fileStorage, err := storage.NewFile(cfg.FileStoragePath, appLogger)
		if err != nil {
			return nil, nil, fmt.Errorf("initialize file storage: %w", err)
		}
		return fileStorage, nil, nil
	}

	return storage.NewMemory(), nil, nil
}
