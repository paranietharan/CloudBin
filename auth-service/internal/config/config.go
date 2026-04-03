package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DBDSN         string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	OTPTTL        string
	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  string
}

func Load() Config {

	return Config{
		Port:          getenv("AUTH_PORT", "8081"),
		DBDSN:         getenv("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/auth_db?sslmode=disable"),
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getenv("REDIS_PASSWORD", ""),
		RedisDB:       getenvInt("REDIS_DB", 0),
		OTPTTL:        getenv("OTP_TTL", "5m"),
		JWTSecret:     getenv("JWT_SECRET", "change-me"),
		JWTIssuer:     getenv("JWT_ISSUER", "cloudbin-auth"),
		JWTAccessTTL:  getenv("JWT_ACCESS_TTL", "24h"),
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
