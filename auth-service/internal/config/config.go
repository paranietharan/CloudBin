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
	DEVMode       bool
	OTPTTL        string
	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  string
	SMTPServer    string
	SMTPPort      int
	SMTPLogin     string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string
}

func Load() Config {

	return Config{
		Port:          getenv("AUTH_PORT", "8081"),
		DBDSN:         getenv("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/auth_db?sslmode=disable"),
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getenv("REDIS_PASSWORD", ""),
		RedisDB:       getenvInt("REDIS_DB", 0),
		DEVMode:       getenvBool("DEV_MODE", false),
		OTPTTL:        getenv("OTP_TTL", "5m"),
		JWTSecret:     getenv("JWT_SECRET", "change-me"),
		JWTIssuer:     getenv("JWT_ISSUER", "cloudbin-auth"),
		JWTAccessTTL:  getenv("JWT_ACCESS_TTL", "24h"),
		SMTPServer:    getenv("SMTP_SERVER", "smtp-relay.brevo.com"),
		SMTPPort:      getenvInt("SMTP_PORT", 587),
		SMTPLogin:     getenv("SMTP_LOGIN", getenv("SMTP_HOST", "")),
		SMTPPassword:  getenv("SMTP_PASSWORD", getenv("SMTP_KEY", "")),
		SMTPFromEmail: getenv("SMTP_FROM_EMAIL", getenv("BREVO_FROM_EMAIL", "no-reply@cloudbin.local")),
		SMTPFromName:  getenv("SMTP_FROM_NAME", getenv("BREVO_FROM_NAME", "CloudBin")),
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

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
