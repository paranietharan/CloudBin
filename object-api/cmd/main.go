package main

import (
	"log"

	"cloudbin-object-api/internal/config"
	"cloudbin-object-api/internal/server"
	"cloudbin-shared/env"
	"cloudbin-shared/httpserver"
)

func main() {
	env.LoadFile(".env")
	env.LoadFile("../.env")

	cfg := config.Load()
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("object api init failed: %v", err)
	}

	log.Printf("object api listening on :%s", cfg.Port)
	if err := httpserver.Run(srv); err != nil {
		log.Fatalf("object-api stopped: %v", err)
	}
}
