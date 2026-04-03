package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cloudbin-storage-node/internal/config"
	"cloudbin-storage-node/internal/storage"
)

func New(cfg config.Config) (*http.Server, error) {
	store, err := storage.New(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "node_id": cfg.NodeID})
	})

	mux.HandleFunc("/objects/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/objects/")
		if key == "" {
			respondError(w, http.StatusBadRequest, "object key is required")
			return
		}

		switch r.Method {
		case http.MethodPut:
			size, err := store.Put(key, r.Body)
			if err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			respondJSON(w, http.StatusCreated, map[string]any{"message": "stored", "size_bytes": size, "node_id": cfg.NodeID})
		case http.MethodGet:
			f, err := store.Get(key)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					respondError(w, http.StatusNotFound, "object not found")
					return
				}
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			defer f.Close()
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, f)
		case http.MethodDelete:
			err := store.Delete(key)
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					respondError(w, http.StatusNotFound, "object not found")
					return
				}
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			respondJSON(w, http.StatusOK, map[string]string{"message": "deleted", "node_id": cfg.NodeID})
		default:
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, nil
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

