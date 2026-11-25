package config

import (
	"strings"

	"cloudbin-shared/env"
)

type Config struct {
	Port              string
	DBDSN             string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	StorageNodes      []string
	ReplicationFactor int
	JWTSecret         string
	JWTIssuer         string
}

func Load() Config {
	nodes := parseNodeList(env.String("STORAGE_NODES", "localhost:8083,localhost:8084,localhost:8085"))

	return Config{
		Port:              env.String("OBJECT_API_PORT", "8082"),
		DBDSN:             env.String("OBJECT_DB_DSN", "postgres://postgres:postgres@localhost:5432/object_db?sslmode=disable"),
		RedisAddr:         env.String("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     env.String("REDIS_PASSWORD", ""),
		RedisDB:           env.Int("REDIS_DB", 0),
		StorageNodes:      nodes,
		ReplicationFactor: env.Int("REPLICATION_FACTOR", 2),
		JWTSecret:         env.String("JWT_SECRET", "change-me"),
		JWTIssuer:         env.String("JWT_ISSUER", "cloudbin-auth"),
	}
}

func parseNodeList(raw string) []string {
	parts := strings.Split(raw, ",")
	nodes := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			v = "http://" + v
		}
		nodes = append(nodes, v)
	}
	return nodes
}
