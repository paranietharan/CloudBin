package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port              string
	DBDSN             string
	StorageNodes      []string
	ReplicationFactor int
	JWTSecret         string
	JWTIssuer         string
}

func Load() Config {
	nodes := parseNodeList(getenv("STORAGE_NODES", "localhost:8083,localhost:8084,localhost:8085"))

	return Config{
		Port:              getenv("OBJECT_API_PORT", "8082"),
		DBDSN:             getenv("OBJECT_DB_DSN", "postgres://postgres:postgres@localhost:5432/object_db?sslmode=disable"),
		StorageNodes:      nodes,
		ReplicationFactor: getenvInt("REPLICATION_FACTOR", 2),
		JWTSecret:         getenv("JWT_SECRET", "change-me"),
		JWTIssuer:         getenv("JWT_ISSUER", "cloudbin-auth"),
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
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

