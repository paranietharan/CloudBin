package config

import "cloudbin-shared/env"

type Config struct {
	Port   string
	Root   string
	NodeID string
}

func Load() Config {
	return Config{
		Port:   env.String("STORAGE_PORT", "8083"),
		Root:   env.String("STORAGE_ROOT", "./data"),
		NodeID: env.String("STORAGE_NODE_ID", "node-1"),
	}
}
