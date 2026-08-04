package main

import (
	"log"
	"net/http"

	"github.com/mikkims/ya/internal/config"
	"github.com/mikkims/ya/internal/handler"
)

func main() {
	cfg := config.Parse()

	err := http.ListenAndServe(cfg.ServerAddress, handler.NewRouter(cfg.BaseURL))
	if err != nil {
		log.Fatal(err)
	}
}
