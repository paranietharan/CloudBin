package main

import (
	"log"

	"cloudbin-auth-service/internal/config"
	"cloudbin-auth-service/internal/server"
	"cloudbin-shared/env"
	"cloudbin-shared/httpserver"
)

func main() {
	env.LoadFile(".env")
	env.LoadFile("../.env")

	cfg := config.Load()

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("auth service init failed: %v", err)
	}

	log.Printf("auth service listening on :%s", cfg.Port)
	if err := httpserver.Run(srv); err != nil {
		log.Fatalf("auth-service stopped: %v", err)
	}
}
