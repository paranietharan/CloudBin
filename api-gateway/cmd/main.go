package main

import (
	"log"

	"cloudbin-api-gateway/internal/config"
	"cloudbin-api-gateway/internal/server"
	"cloudbin-shared/env"
	"cloudbin-shared/httpserver"
)

func main() {
	env.LoadFile(".env")
	env.LoadFile("../.env")

	cfg := config.Load()
	srv := server.New(cfg)

	log.Printf("api gateway listening on :%s", cfg.Port)
	if err := httpserver.Run(srv); err != nil {
		log.Fatalf("api-gateway stopped: %v", err)
	}
}
