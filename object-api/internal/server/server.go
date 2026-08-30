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
	"github.com/redis/go-redis/v9"
)

type Server struct {
	http *http.Server
	db   *pgxpool.Pool
	rdb  *redis.Client
}

func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.http.Shutdown(ctx)
	if s.db != nil {
		s.db.Close()
	}
	if s.rdb != nil {
		_ = s.rdb.Close()
	}
	return err
}

func New(cfg config.Config) (*Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("connect object db: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping object db: %w", err)
	}

	var rdb *redis.Client
	if cfg.RedisAddr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err := rdb.Ping(ctx).Err(); err != nil {
			// Log but don't crash if optional Redis has issues, fallback to in-memory
			_ = rdb.Close()
			rdb = nil
		}
	}

	repo := repository.NewPostgres(db)
	objectService, err := service.New(repo, cfg.StorageNodes, cfg.ReplicationFactor, rdb)
	if err != nil {
		db.Close()
		if rdb != nil {
			_ = rdb.Close()
		}
		return nil, err
	}
	httpHandler := handler.NewHTTP(objectService, cfg.JWTSecret, cfg.JWTIssuer, rdb)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	return &Server{
		http: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      mux,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		db:  db,
		rdb: rdb,
	}, nil
}
