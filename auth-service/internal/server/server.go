package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloudbin-auth-service/internal/auth/handler"
	"cloudbin-auth-service/internal/auth/otp"
	"cloudbin-auth-service/internal/auth/repository"
	"cloudbin-auth-service/internal/auth/service"
	"cloudbin-auth-service/internal/config"

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
		return nil, fmt.Errorf("connect auth db: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping auth db: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	otpTTL, err := time.ParseDuration(cfg.OTPTTL)
	if err != nil {
		otpTTL = 5 * time.Minute
	}
	jwtTTL, err := time.ParseDuration(cfg.JWTAccessTTL)
	if err != nil {
		jwtTTL = 24 * time.Hour
	}

	repo := repository.NewPostgres(db)
	otpStore := otp.NewStore(rdb, otpTTL)
	authService := service.New(
		repo,
		otpStore,
		rdb,
		cfg.JWTSecret,
		cfg.JWTIssuer,
		jwtTTL,
		cfg.DEVMode,
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUser,
		cfg.SMTPPassword,
		cfg.SMTPFromEmail,
		cfg.SMTPFromName,
	)
	httpHandler := handler.NewHTTP(authService, cfg.JWTSecret, cfg.JWTIssuer)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	return &Server{
		http: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		db:  db,
		rdb: rdb,
	}, nil
}
