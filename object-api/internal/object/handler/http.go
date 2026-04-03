package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"cloudbin-object-api/internal/object/service"

	"github.com/golang-jwt/jwt/v5"
)

type HTTP struct {
	svc       *service.Service
	jwtSecret string
	jwtIssuer string
}

type principal struct {
	UserID string
	Role   string
}

type principalContextKey struct{}

type keyRequest struct {
	ObjectKey string `json:"object_key"`
	OwnerID   string `json:"owner_id"`
}

func NewHTTP(svc *service.Service, jwtSecret, jwtIssuer string) *HTTP {
	return &HTTP{svc: svc, jwtSecret: jwtSecret, jwtIssuer: jwtIssuer}
}

func (h *HTTP) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/objects/", h.authMiddleware(h.handleObjects))
	mux.HandleFunc("/admin/objects/", h.authMiddleware(h.handleAdminObjects))

	mux.HandleFunc("/api/v1/upload-file", h.authMiddleware(h.handleUploadAlias))
	mux.HandleFunc("/api/v1/download-file", h.authMiddleware(h.handleDownloadAlias))
	mux.HandleFunc("/api/v1/delete-file", h.authMiddleware(h.handleDeleteAlias))
	mux.HandleFunc("/api/v1/hide-file", h.authMiddleware(h.handleHideAlias))
	mux.HandleFunc("/api/v1/admin/hide-file", h.authMiddleware(h.handleAdminHideAlias))
	mux.HandleFunc("/api/v1/admin/delete-file", h.authMiddleware(h.handleAdminDeleteAlias))
	mux.HandleFunc("/api/v1/get-user-files", h.authMiddleware(h.handleGetUserFiles))
}

func (h *HTTP) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HTTP) handleObjects(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/objects/")
	if key == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}

	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if strings.HasSuffix(key, "/hide") {
		if r.Method != http.MethodPut {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		key = strings.TrimSuffix(key, "/hide")
		hideCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := h.svc.Hide(hideCtx, p.UserID, key, p.UserID, p.Role, "owner hide") ; err != nil {
			h.mapError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "object hidden"})
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.uploadObject(w, r, p.UserID, key)
	case http.MethodGet:
		h.downloadObject(w, r, p.UserID, key)
	case http.MethodDelete:
		h.deleteObject(w, r, p.UserID, key)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTP) handleAdminObjects(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isPrivileged(p) {
		respondError(w, http.StatusForbidden, "admin route requires admin or owner role")
		return
	}

	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if ownerID == "" {
		respondError(w, http.StatusBadRequest, "owner_id is required")
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/admin/objects/")
	if key == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}

	if strings.HasSuffix(key, "/hide") {
		if r.Method != http.MethodPut {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		key = strings.TrimSuffix(key, "/hide")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := h.svc.Hide(ctx, ownerID, key, p.UserID, p.Role, "admin hide") ; err != nil {
			h.mapError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"message": "object hidden"})
		return
	}

	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	h.deleteObject(w, r, ownerID, key)
}

func (h *HTTP) handleUploadAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	key := readObjectKey(r)
	h.uploadObject(w, r, p.UserID, key)
}

func (h *HTTP) handleDownloadAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	key := readObjectKey(r)
	h.downloadObject(w, r, p.UserID, key)
}

func (h *HTTP) handleDeleteAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	key := readObjectKey(r)
	h.deleteObject(w, r, p.UserID, key)
}

func (h *HTTP) handleHideAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	key := readObjectKey(r)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Hide(ctx, p.UserID, key, p.UserID, p.Role, "owner hide"); err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "object hidden"})
}

func (h *HTTP) handleAdminHideAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isPrivileged(p) {
		respondError(w, http.StatusForbidden, "admin route requires admin or owner role")
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if ownerID == "" {
		bodyOwner := readOwnerID(r)
		ownerID = bodyOwner
	}
	if ownerID == "" {
		respondError(w, http.StatusBadRequest, "owner_id is required")
		return
	}
	key := readObjectKey(r)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Hide(ctx, ownerID, key, p.UserID, p.Role, "admin hide"); err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "object hidden"})
}

func (h *HTTP) handleAdminDeleteAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !isPrivileged(p) {
		respondError(w, http.StatusForbidden, "admin route requires admin or owner role")
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if ownerID == "" {
		ownerID = readOwnerID(r)
	}
	if ownerID == "" {
		respondError(w, http.StatusBadRequest, "owner_id is required")
		return
	}
	key := readObjectKey(r)
	h.deleteObject(w, r, ownerID, key)
}

func (h *HTTP) handleGetUserFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	files, err := h.svc.ListOwnerObjects(ctx, p.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (h *HTTP) uploadObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := h.svc.Upload(ctx, ownerID, key, r.Header.Get("Content-Type"), data); err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"message": "object stored"})
}

func (h *HTTP) downloadObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	rec, data, err := h.svc.Download(ctx, ownerID, key)
	if err != nil {
		h.mapError(w, err)
		return
	}

	if rec.ContentType != "" {
		w.Header().Set("Content-Type", rec.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if rec.ETag != "" {
		w.Header().Set("ETag", rec.ETag)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *HTTP) deleteObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.Delete(ctx, ownerID, key); err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "object deleted"})
}

func (h *HTTP) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
		if token == "" {
			respondError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(h.jwtSecret), nil
		})
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		subject, _ := claims["sub"].(string)
		issuer, _ := claims["iss"].(string)
		role, _ := claims["role"].(string)
		if subject == "" || issuer != h.jwtIssuer {
			respondError(w, http.StatusUnauthorized, "invalid token issuer")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal{UserID: subject, Role: role})))
	}
}

func principalFromContext(ctx context.Context) (principal, bool) {
	v := ctx.Value(principalContextKey{})
	if v == nil {
		return principal{}, false
	}
	p, ok := v.(principal)
	return p, ok
}

func isPrivileged(p principal) bool {
	return p.Role == "admin" || p.Role == "owner"
}

func readObjectKey(r *http.Request) string {
	key := strings.TrimSpace(r.URL.Query().Get("object_key"))
	if key != "" {
		return key
	}

	if r.Body == nil {
		return ""
	}
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(strings.NewReader(string(buf)))

	var req keyRequest
	if err := json.Unmarshal(buf, &req); err != nil {
		return ""
	}
	return strings.TrimSpace(req.ObjectKey)
}

func readOwnerID(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(strings.NewReader(string(buf)))

	var req keyRequest
	if err := json.Unmarshal(buf, &req); err != nil {
		return ""
	}
	return strings.TrimSpace(req.OwnerID)
}

func (h *HTTP) mapError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, service.ErrInvalidKey):
		respondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrForbidden):
		respondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrReplicationWriteFail):
		respondError(w, http.StatusBadGateway, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

