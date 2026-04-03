package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"

	"cloudbin-api-gateway/internal/config"
	"cloudbin-api-gateway/internal/server"
)

func main() {
	root, err := projectRoot()
	if err != nil {
		log.Fatalf("failed to read working directory: %v", err)
	}
	loadDotEnv(filepath.Join(root, "api-gateway", ".env"))
	loadDotEnv(filepath.Join(root, ".env"))

	cfg := config.Load()
	srv := server.New(cfg)

	log.Printf("api gateway listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("gateway stopped: %v", err)
	}
}

func projectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if filepath.Base(cwd) == "migrations" {
		return filepath.Dir(cwd), nil
	}
	return cwd, nil
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("warning: failed to scan %s: %v", path, err)
	}
}
