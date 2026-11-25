package main

import (
	"log"
	"os"
	"path/filepath"

	"cloudbin-shared/env"
	"cloudbin-shared/httpserver"
	"cloudbin-storage-node/internal/config"
	"cloudbin-storage-node/internal/server"
)

func main() {
	root, err := projectRoot()
	if err != nil {
		log.Fatalf("failed to read working directory: %v", err)
	}
	env.LoadFile(filepath.Join(root, "storage-node", ".env"))
	env.LoadFile(filepath.Join(root, ".env"))

	cfg := config.Load()
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("storage node init failed: %v", err)
	}

	log.Printf("storage node %s listening on :%s", cfg.NodeID, cfg.Port)
	if err := httpserver.Run(srv); err != nil {
		log.Fatalf("storage-node stopped: %v", err)
	}
}

func projectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd, nil
}
