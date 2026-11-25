package handler

import (
	"context"
	"net/http"
	"time"
)

func (h *HTTP) handleUploadAlias(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodPut, http.MethodPost) {
		return
	}
	if !h.allowByUserAndIP(r, p.UserID, "upload", 30, time.Minute) {
		respondError(w, http.StatusTooManyRequests, "rate limit exceeded for upload")
		return
	}
	key := readObjectKey(r)
	h.uploadObject(w, r, p.UserID, key)
}

func (h *HTTP) handleDownloadAlias(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	key := readObjectKey(r)
	h.downloadObject(w, r, p.UserID, key)
}

func (h *HTTP) handleDeleteAlias(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodDelete) {
		return
	}
	if !h.allowByUserAndIP(r, p.UserID, "delete", 40, time.Minute) {
		respondError(w, http.StatusTooManyRequests, "rate limit exceeded for delete")
		return
	}
	key := readObjectKey(r)
	h.deleteObject(w, r, p.UserID, key)
}

func (h *HTTP) handleFileExists(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	h.objectExists(w, r, p.UserID, readObjectKey(r))
}

func (h *HTTP) handleHideAlias(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	key := readObjectKey(r)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Hide(ctx, p.UserID, key, p.UserID, p.Role, "owner hide"); err != nil {
		respondServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"message": "object hidden"})
}

func (h *HTTP) handleAdminHideAlias(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	ownerID := adminOwnerID(w, r, p)
	if ownerID == "" {
		return
	}
	key := readObjectKey(r)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Hide(ctx, ownerID, key, p.UserID, p.Role, "admin hide"); err != nil {
		respondServiceError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"message": "object hidden"})
}

func (h *HTTP) handleAdminDeleteAlias(w http.ResponseWriter, r *http.Request, p principal) {
	if !allowMethod(w, r, http.MethodDelete) {
		return
	}
	ownerID := adminOwnerID(w, r, p)
	if ownerID == "" {
		return
	}
	key := readObjectKey(r)
	h.deleteObject(w, r, ownerID, key)
}

func (h *HTTP) permissionAlias(permission string) authenticatedHandler {
	return func(w http.ResponseWriter, r *http.Request, p principal) {
		if !allowMethod(w, r, http.MethodPut) {
			return
		}
		if !h.allowByUserAndIP(r, p.UserID, "permission", 60, time.Minute) {
			respondError(w, http.StatusTooManyRequests, "rate limit exceeded for permission updates")
			return
		}
		key := readObjectKey(r)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := h.svc.SetPermission(ctx, p.UserID, key, permission); err != nil {
			respondServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"message": "permission set to " + permission})
	}
}
