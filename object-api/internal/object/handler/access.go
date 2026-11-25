package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"cloudbin-shared/response"
)

type permissionRequest struct {
	Permission string `json:"permission"`
}

type visibilityRequest struct {
	Visibility string `json:"visibility"`
	Reason     string `json:"reason"`
}

func (h *HTTP) handleObjectPermission(w http.ResponseWriter, r *http.Request, userID, key string) {
	if !allowMethod(w, r, http.MethodPut, http.MethodPatch) {
		return
	}
	if !h.allowByUserAndIP(r, userID, "permission", 60, time.Minute) {
		response.Error(w, http.StatusTooManyRequests, "rate limit exceeded for permission updates")
		return
	}

	perm := strings.TrimSpace(r.URL.Query().Get("permission"))
	if perm == "" && r.Body != nil && r.ContentLength > 0 {
		var req permissionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			perm = strings.TrimSpace(req.Permission)
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.svc.SetPermission(ctx, userID, key, perm); err != nil {
		respondServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"message": "permission set to " + perm, "permission": perm})
}

func (h *HTTP) handleObjectVisibility(w http.ResponseWriter, r *http.Request, actor principal, ownerID, key, reason string) {
	if !allowMethod(w, r, http.MethodPut, http.MethodPatch, http.MethodPost) {
		return
	}

	if r.Body != nil && r.ContentLength > 0 {
		var req visibilityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Reason != "" {
			reason = req.Reason
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Hide(ctx, ownerID, key, actor.UserID, actor.Role, reason); err != nil {
		respondServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"message": "object hidden", "visibility": "hidden"})
}

func (h *HTTP) handleAdminObjects(w http.ResponseWriter, r *http.Request, p principal) {
	ownerID := adminOwnerID(w, r, p)
	if ownerID == "" {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	key := strings.TrimPrefix(path, "/admin/objects/")
	if key == "" {
		key = readObjectKey(r)
	}
	if key == "" {
		response.Error(w, http.StatusBadRequest, "object key is required")
		return
	}

	if isVisibilityPath(key) {
		h.handleObjectVisibility(w, r, p, ownerID, visibilityKey(key), "admin hide")
		return
	}

	if !allowMethod(w, r, http.MethodDelete) {
		return
	}
	h.deleteObject(w, r, ownerID, key)
}

func adminOwnerID(w http.ResponseWriter, r *http.Request, p principal) string {
	if !isPrivileged(p) {
		response.Error(w, http.StatusForbidden, "admin route requires admin or owner role")
		return ""
	}

	ownerID := readOwnerID(r)
	if ownerID == "" {
		response.Error(w, http.StatusBadRequest, "owner_id is required")
		return ""
	}
	return ownerID
}

func isPrivileged(p principal) bool {
	return p.Role == "admin" || p.Role == "owner"
}
