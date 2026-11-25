package config

import (
	"log"

	"cloudbin-shared/env"
)

type Config struct {
	Port           string
	AuthServiceURL string
	ObjectAPIURL   string
	JWTSecret      string
	JWTIssuer      string
}

func Load() Config {
	devMode := env.Bool("DEV_MODE", false)
	secret := env.String("JWT_SECRET", "change-me")

	if !devMode && (secret == "change-me" || len(secret) < 16) {
		log.Printf("WARNING: Insecure or default JWT_SECRET used in non-development mode")
	}

	return Config{
		Port:           env.String("GATEWAY_PORT", "8080"),
		AuthServiceURL: env.String("AUTH_SERVICE_URL", "http://localhost:8081"),
		ObjectAPIURL:   env.String("OBJECT_API_URL", "http://localhost:8082"),
		JWTSecret:      secret,
		JWTIssuer:      env.String("JWT_ISSUER", "cloudbin-auth"),
	}
}
