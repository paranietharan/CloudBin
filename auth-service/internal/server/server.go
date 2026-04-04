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

func New(cfg config.Config) (*http.Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("connect auth db: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping auth db: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
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
		cfg.JWTSecret,
		cfg.JWTIssuer,
		jwtTTL,
		cfg.SMTPServer,
		cfg.SMTPPort,
		cfg.SMTPLogin,
		cfg.SMTPPassword,
		cfg.SMTPFromEmail,
		cfg.SMTPFromName,
	)
	httpHandler := handler.NewHTTP(authService, cfg.JWTSecret, cfg.JWTIssuer)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, nil
}
