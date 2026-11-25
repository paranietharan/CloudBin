package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxUploadBytes = 20 << 20 // 20 MiB

func (h *HTTP) uploadObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		key = uuid.NewString()
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(w, http.StatusRequestEntityTooLarge, "upload exceeds maximum size of 20 MiB")
			return
		}
		respondError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := h.svc.Upload(ctx, ownerID, key, r.Header.Get("Content-Type"), data); err != nil {
		respondServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"message": "object stored", "object_key": key})
}

func (h *HTTP) downloadObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rec, stream, err := h.svc.Download(ctx, ownerID, key)
	writeDownload(w, rec, stream, err)
}

func (h *HTTP) headObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	rec, stream, err := h.svc.Download(ctx, ownerID, key)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	_ = stream.Close()

	setObjectHeaders(w, rec)
	w.Header().Set("Content-Length", strconv.FormatInt(rec.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)
}

func (h *HTTP) deleteObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Delete(ctx, ownerID, key); err != nil {
		respondServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"message": "object deleted"})
}

func (h *HTTP) objectExists(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	exists, err := h.svc.FileExists(ctx, ownerID, key)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"object_key": key, "exists": exists})
}

func (h *HTTP) handleGetUserFiles(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	query := r.URL.Query()
	limit := parseIntOrDefault(query.Get("limit"), 20)
	offset := parseIntOrDefault(query.Get("offset"), 0)
	page, err := h.svc.ListOwnerObjectsPage(ctx, p.UserID, query.Get("permission"), query.Get("visibility"), query.Get("key"), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"files":  page.Files,
		"total":  page.Total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

func parseIntOrDefault(raw string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return parsed
}
