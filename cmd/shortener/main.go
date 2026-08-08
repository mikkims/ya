package main

import (
	"log"
	"net/http"

	"github.com/mikkims/ya/internal/config"
	"github.com/mikkims/ya/internal/handler"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
)

func main() {
	cfg := config.Load()
	urlStorage := storage.NewMemory()
	shortenerService := service.NewShortener(urlStorage)

	err := http.ListenAndServe(cfg.ServerAddress, handler.NewRouter(cfg.BaseURL, shortenerService))
	if err != nil {
		log.Fatal(err)
	}
}
