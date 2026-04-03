package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloudbin-object-api/internal/config"
	"cloudbin-object-api/internal/object/handler"
	"cloudbin-object-api/internal/object/repository"
	"cloudbin-object-api/internal/object/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func New(cfg config.Config) (*http.Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("connect object db: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping object db: %w", err)
	}

	repo := repository.NewPostgres(db)
	objectService, err := service.New(repo, cfg.StorageNodes, cfg.ReplicationFactor)
	if err != nil {
		return nil, err
	}
	httpHandler := handler.NewHTTP(objectService, cfg.JWTSecret, cfg.JWTIssuer)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, nil
}

