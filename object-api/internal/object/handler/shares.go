package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloudbin-shared/response"
)

type shareLinkRequest struct {
	ObjectKey       string `json:"object_key"`
	ExpiresInSecond int    `json:"expires_in_seconds"`
}

func (h *HTTP) handleShares(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.authMiddleware(h.handleCreateShareLink)(w, r)
	case http.MethodGet:
		h.handleDownloadByShareLink(w, r)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTP) handleCreateShareLink(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	var req shareLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.createShareLink(w, r, p.UserID, req.ObjectKey, req.ExpiresInSecond)
}

func (h *HTTP) handleCreateShareLinkForKey(w http.ResponseWriter, r *http.Request, userID, key string) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	seconds := 0
	if r.Body != nil && r.ContentLength > 0 {
		var req shareLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			seconds = req.ExpiresInSecond
		}
	} else {
		if value, err := strconv.Atoi(r.URL.Query().Get("expires_in_seconds")); err == nil {
			seconds = value
		}
	}
	h.createShareLink(w, r, userID, key, seconds)
}

func (h *HTTP) createShareLink(w http.ResponseWriter, r *http.Request, userID, key string, seconds int) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	var ttl time.Duration
	if seconds > 0 {
		ttl = time.Duration(seconds) * time.Second
	}
	token, expiresAt, err := h.svc.CreateTemporaryShareLink(ctx, userID, key, ttl)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"expires_at": expiresAt,
		"url":        "/api/v1/shares/" + token,
	})
}

func (h *HTTP) handleDownloadByShareLink(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		if path, ok := strings.CutPrefix(r.URL.Path, "/api/v1/shares/"); ok {
			token = strings.Trim(path, "/")
		}
	}
	if token == "" {
		response.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rec, stream, err := h.svc.DownloadByShareToken(ctx, token)
	writeDownload(w, rec, stream, err)
}

func (h *HTTP) handlePublicDownload(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	ownerID := readOwnerID(r)

	key := strings.TrimSpace(r.URL.Query().Get("object_key"))
	if key == "" {
		key = strings.TrimSpace(r.URL.Query().Get("key"))
	}
	if key == "" {
		if path, ok := strings.CutPrefix(r.URL.Path, "/api/v1/public/objects/"); ok {
			key = strings.Trim(path, "/")
		}
	}
	if key == "" {
		key = readObjectKey(r)
	}

	if ownerID == "" || key == "" {
		response.Error(w, http.StatusBadRequest, "owner_id and object_key are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rec, stream, err := h.svc.DownloadPublic(ctx, ownerID, key)
	writeDownload(w, rec, stream, err)
}
