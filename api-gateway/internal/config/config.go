package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port           string
	AuthServiceURL string
	ObjectAPIURL   string
	JWTSecret      string
	JWTIssuer      string
	DEVMode        bool
}

func Load() Config {
	devMode := getenvBool("DEV_MODE", false)
	secret := getenv("JWT_SECRET", "change-me")

	if !devMode && (secret == "change-me" || len(secret) < 16) {
		log.Printf("WARNING: Insecure or default JWT_SECRET used in non-development mode")
	}

	return Config{
		Port:           getenv("GATEWAY_PORT", "8080"),
		AuthServiceURL: getenv("AUTH_SERVICE_URL", "http://localhost:8081"),
		ObjectAPIURL:   getenv("OBJECT_API_URL", "http://localhost:8082"),
		JWTSecret:      secret,
		JWTIssuer:      getenv("JWT_ISSUER", "cloudbin-auth"),
		DEVMode:        devMode,
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
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
