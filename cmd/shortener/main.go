package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/mikkims/ya/internal/audit"
	"github.com/mikkims/ya/internal/config"
	appdatabase "github.com/mikkims/ya/internal/database"
	appgzip "github.com/mikkims/ya/internal/gzip"
	"github.com/mikkims/ya/internal/handler"
	"github.com/mikkims/ya/internal/logger"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
	"github.com/mikkims/ya/migrations"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

func main() {
	cfg := config.Load()
	appLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	publisher, auditFile, err := buildAudit(cfg)
	if err != nil {
		appLogger.Error().Msg("failed to initialize audit")
		return
	}
	if auditFile != nil {
		defer func() {
			if err := auditFile.Close(); err != nil {
				appLogger.Error().Msg("failed to close audit file")
			}
		}()
	}
	appContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	startupContext, cancelStartup := context.WithTimeout(appContext, 10*time.Second)
	defer cancelStartup()
	urlStorage, database, err := buildStorage(startupContext, cfg, appLogger)
	if err != nil {
		appLogger.Error().Err(err).Msg("failed to initialize storage")
		return
	}
	shortenerService := service.NewShortener(urlStorage, service.DeleteOptions{
		BufferSize:    cfg.DeleteBufferSize,
		FlushInterval: cfg.DeleteFlushInterval,
	})
	if database != nil {
		defer func() {
			if err := database.Close(); err != nil {
				appLogger.Error().Err(err).Msg("failed to close database")
			}
		}()
	}

	var pinger handler.Pinger
	if database != nil {
		pinger = database
	}
	router := handler.NewRouterWithAudit(cfg.BaseURL, shortenerService, pinger, publisher)
	compressedRouter := appgzip.MiddlewareGzip(appLogger)(router)

	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: logger.Middleware(appLogger)(compressedRouter),
	}
	group, groupContext := errgroup.WithContext(appContext)
	group.Go(func() error {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	group.Go(func() error {
		return shortenerService.RunDeleteWorker(groupContext)
	})
	group.Go(func() error {
		<-groupContext.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	})
	if err := group.Wait(); err != nil {
		appLogger.Error().Err(err).Msg("server stopped with error")
	}
}

func buildAudit(cfg *config.Config) (*audit.Publisher, *audit.File, error) {
	var observers []audit.Observer
	var file *audit.File
	if cfg.AuditFile != "" {
		var err error
		file, err = audit.NewFile(cfg.AuditFile)
		if err != nil {
			return nil, nil, err
		}
		observers = append(observers, file)
	}
	if cfg.AuditURL != "" {
		receiver, err := audit.NewHTTP(cfg.AuditURL)
		if err != nil {
			if file != nil {
				_ = file.Close()
			}
			return nil, nil, err
		}
		observers = append(observers, receiver)
	}
	return audit.NewPublisher(observers...), file, nil
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
