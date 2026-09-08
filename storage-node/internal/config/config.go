package config

import "os"

type Config struct {
	Port   string
	Root   string
	NodeID string
}

func Load() Config {
	return Config{
		Port:   getenv("STORAGE_PORT", "8083"),
		Root:   getenv("STORAGE_ROOT", "./data"),
		NodeID: getenv("STORAGE_NODE_ID", "node-1"),
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
