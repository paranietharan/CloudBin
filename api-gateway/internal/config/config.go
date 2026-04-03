package config

import "os"

type Config struct {
    Port           string
    AuthServiceURL string
    ObjectAPIURL   string
    JWTSecret      string
    JWTIssuer      string
}

func Load() Config {
    return Config{
        Port:           getenv("GATEWAY_PORT", "8080"),
        AuthServiceURL: getenv("AUTH_SERVICE_URL", "http://localhost:8081"),
        ObjectAPIURL:   getenv("OBJECT_API_URL", "http://localhost:8082"),
        JWTSecret:      getenv("JWT_SECRET", "change-me"),
        JWTIssuer:      getenv("JWT_ISSUER", "cloudbin-auth"),
    }
}

func getenv(key, fallback string) string {
    v := os.Getenv(key)
    if v == "" {
        return fallback
    }
    return v
}
