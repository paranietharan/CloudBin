package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloudbin-object-api/internal/object/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type HTTP struct {
	svc         *service.Service
	jwtSecret   string
	jwtIssuer   string
	redisClient *redis.Client
	limiter     *fixedWindowLimiter
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

type shareLinkRequest struct {
	ObjectKey       string `json:"object_key"`
	ExpiresInSecond int    `json:"expires_in_seconds"`
}

const maxUploadBytes = 20 << 20 // 20 MiB

type windowCounter struct {
	windowStart time.Time
	count       int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]windowCounter
}

func NewHTTP(svc *service.Service, jwtSecret, jwtIssuer string, redisClient *redis.Client) *HTTP {
	return &HTTP{
		svc:         svc,
		jwtSecret:   jwtSecret,
		jwtIssuer:   jwtIssuer,
		redisClient: redisClient,
		limiter:     &fixedWindowLimiter{entries: make(map[string]windowCounter)},
	}
}

func (l *fixedWindowLimiter) Allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// Periodic cleanup of stale entries to prevent memory leak
	if len(l.entries) > 1000 {
		for k, v := range l.entries {
			if now.Sub(v.windowStart) >= window*2 {
				delete(l.entries, k)
			}
		}
	}

	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.windowStart) >= window {
		l.entries[key] = windowCounter{windowStart: now, count: 1}
		return true
	}

	if entry.count >= limit {
		return false
	}

	entry.count++
	l.entries[key] = entry
	return true
}

func (h *HTTP) allowByUserAndIP(r *http.Request, userID, action string, limit int, window time.Duration) bool {
	ip := "unknown"
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		ip = host
	}
	if ip == "" {
		ip = "unknown"
	}

	if h.redisClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		return h.allowRedis(ctx, fmt.Sprintf("ratelimit:user:%s:%s", userID, action), limit, window) &&
			h.allowRedis(ctx, fmt.Sprintf("ratelimit:ip:%s:%s", ip, action), limit, window)
	}

	return h.limiter.Allow(userID+":"+action, limit, window) && h.limiter.Allow(ip+":"+action, limit, window)
}

func (h *HTTP) allowRedis(ctx context.Context, key string, limit int, window time.Duration) bool {
	cnt, err := h.redisClient.Incr(ctx, key).Result()
	if err != nil {
		return true // Fail open on Redis error
	}
	if cnt == 1 {
		_ = h.redisClient.Expire(ctx, key, window).Err()
	}
	return cnt <= int64(limit)
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
	mux.HandleFunc("/api/v1/file-exists", h.authMiddleware(h.handleFileExists))
	mux.HandleFunc("/api/v1/make-private-read", h.authMiddleware(h.handleMakePrivateRead))
	mux.HandleFunc("/api/v1/make-public-read", h.authMiddleware(h.handleMakePublicRead))
	mux.HandleFunc("/api/v1/share-link", h.authMiddleware(h.handleCreateShareLink))
	mux.HandleFunc("/api/v1/share/download", h.handleDownloadByShareLink)
	mux.HandleFunc("/api/v1/public/download-file", h.handlePublicDownload)
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
		if err := h.svc.Hide(hideCtx, p.UserID, key, p.UserID, p.Role, "owner hide"); err != nil {
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
		if err := h.svc.Hide(ctx, ownerID, key, p.UserID, p.Role, "admin hide"); err != nil {
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
	if !h.allowByUserAndIP(r, p.UserID, "upload", 30, time.Minute) {
		respondError(w, http.StatusTooManyRequests, "rate limit exceeded for upload")
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
	if !h.allowByUserAndIP(r, p.UserID, "delete", 40, time.Minute) {
		respondError(w, http.StatusTooManyRequests, "rate limit exceeded for delete")
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
		ownerID = readOwnerID(r)
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

	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 20)
	offset := parseIntOrDefault(r.URL.Query().Get("offset"), 0)
	permission := strings.TrimSpace(r.URL.Query().Get("permission"))
	visibility := strings.TrimSpace(r.URL.Query().Get("visibility"))
	keyQuery := strings.TrimSpace(r.URL.Query().Get("key"))

	files, total, err := h.svc.ListOwnerObjectsPage(ctx, p.UserID, permission, visibility, keyQuery, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"files":  files,
		"total":  total,
		"limit":  normalizeLimit(limit),
		"offset": normalizeOffset(offset),
	})
}

func parseIntOrDefault(raw string, fallback int) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func (h *HTTP) handleFileExists(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	exists, err := h.svc.FileExists(ctx, p.UserID, key)
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"object_key": key, "exists": exists})
}

func (h *HTTP) handleMakePrivateRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !h.allowByUserAndIP(r, p.UserID, "permission", 60, time.Minute) {
		respondError(w, http.StatusTooManyRequests, "rate limit exceeded for permission updates")
		return
	}
	key := readObjectKey(r)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.MakePrivateRead(ctx, p.UserID, key); err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "permission set to private-read"})
}

func (h *HTTP) handleMakePublicRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !h.allowByUserAndIP(r, p.UserID, "permission", 60, time.Minute) {
		respondError(w, http.StatusTooManyRequests, "rate limit exceeded for permission updates")
		return
	}
	key := readObjectKey(r)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := h.svc.MakePublicRead(ctx, p.UserID, key); err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "permission set to public-read"})
}

func (h *HTTP) handlePublicDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	key := readObjectKey(r)
	if ownerID == "" {
		ownerID = readOwnerID(r)
	}
	if ownerID == "" || key == "" {
		respondError(w, http.StatusBadRequest, "owner_id and object_key are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rec, stream, err := h.svc.DownloadPublic(ctx, ownerID, key)
	if err != nil {
		h.mapError(w, err)
		return
	}
	defer stream.Close()

	if rec.ContentType != "" {
		w.Header().Set("Content-Type", rec.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if rec.ETag != "" {
		w.Header().Set("ETag", rec.ETag)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, stream)
}

func (h *HTTP) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req shareLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ttl := 15 * time.Minute
	if req.ExpiresInSecond > 0 {
		ttl = time.Duration(req.ExpiresInSecond) * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	token, expiresAt, err := h.svc.CreateTemporaryShareLink(ctx, p.UserID, req.ObjectKey, ttl)
	if err != nil {
		h.mapError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"expires_at": expiresAt,
		"url":        "/api/v1/share/download?token=" + token,
	})
}

func (h *HTTP) handleDownloadByShareLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rec, stream, err := h.svc.DownloadByShareToken(ctx, token)
	if err != nil {
		h.mapError(w, err)
		return
	}
	defer stream.Close()

	if rec.ContentType != "" {
		w.Header().Set("Content-Type", rec.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if rec.ETag != "" {
		w.Header().Set("ETag", rec.ETag)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, stream)
}

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
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"message": "object stored", "object_key": key})
}

func (h *HTTP) downloadObject(w http.ResponseWriter, r *http.Request, ownerID, key string) {
	if strings.TrimSpace(key) == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	rec, stream, err := h.svc.Download(ctx, ownerID, key)
	if err != nil {
		h.mapError(w, err)
		return
	}
	defer stream.Close()

	if rec.ContentType != "" {
		w.Header().Set("Content-Type", rec.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if rec.ETag != "" {
		w.Header().Set("ETag", rec.ETag)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, stream)
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
		// Prefer verified headers from API Gateway if present
		if headerUserID := strings.TrimSpace(r.Header.Get("X-User-ID")); headerUserID != "" {
			role := strings.TrimSpace(r.Header.Get("X-User-Role"))
			if role == "" {
				role = "user"
			}
			next(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal{UserID: headerUserID, Role: role})))
			return
		}

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
	if headerKey := strings.TrimSpace(r.Header.Get("X-Object-Key")); headerKey != "" {
		return headerKey
	}

	if r.Body == nil || !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
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
	if ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id")); ownerID != "" {
		return ownerID
	}
	if r.Body == nil || !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
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
	success := status >= http.StatusOK && status < http.StatusMultipleChoices
	wrapped := payload
	switch v := payload.(type) {
	case map[string]any:
		if _, ok := v["success"]; !ok {
			v["success"] = success
		}
		wrapped = v
	case map[string]string:
		m := make(map[string]any, len(v)+1)
		for k, val := range v {
			m[k] = val
		}
		m["success"] = success
		wrapped = m
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(wrapped)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message, "error_code": errorCodeFromStatus(status)})
}

func errorCodeFromStatus(status int) string {
	code := strings.ToLower(http.StatusText(status))
	code = strings.ReplaceAll(code, " ", "_")
	if code == "" {
		return "internal_server_error"
	}
	return code
}
