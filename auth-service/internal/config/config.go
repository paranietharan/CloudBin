package config

import "cloudbin-shared/env"

type Config struct {
	Port          string
	DBDSN         string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	DEVMode       bool
	OTPTTL        string
	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  string
	SMTPHost      string
	SMTPPort      int
	SMTPUser      string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string
}

func Load() Config {
	return Config{
		Port:          env.String("AUTH_PORT", "8081"),
		DBDSN:         env.String("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/auth_db?sslmode=disable"),
		RedisAddr:     env.String("REDIS_ADDR", "localhost:6379"),
		RedisPassword: env.String("REDIS_PASSWORD", ""),
		RedisDB:       env.Int("REDIS_DB", 0),
		DEVMode:       env.Bool("DEV_MODE", false),
		OTPTTL:        env.String("OTP_TTL", "5m"),
		JWTSecret:     env.String("JWT_SECRET", "change-me"),
		JWTIssuer:     env.String("JWT_ISSUER", "cloudbin-auth"),
		JWTAccessTTL:  env.String("JWT_ACCESS_TTL", "24h"),
		SMTPHost:      env.String("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:      env.Int("SMTP_PORT", 587),
		SMTPUser:      env.String("SMTP_USER", ""),
		SMTPPassword:  env.String("SMTP_PASSWORD", ""),
		SMTPFromEmail: env.String("SMTP_FROM_EMAIL", env.String("SMTP_USER", "")),
		SMTPFromName:  env.String("SMTP_FROM_NAME", "CloudBin"),
	}
}
