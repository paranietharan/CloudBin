package handler

import (
	"net/http"
	"strings"

	"cloudbin-object-api/internal/object/service"

	"github.com/redis/go-redis/v9"
)

type HTTP struct {
	svc         *service.Service
	jwtSecret   string
	jwtIssuer   string
	redisClient *redis.Client
	limiter     *fixedWindowLimiter
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

func (h *HTTP) RegisterRoutes(mux *http.ServeMux) {
	protected := func(path string, handle authenticatedHandler) {
		mux.HandleFunc(path, h.authMiddleware(handle))
	}
	mux.HandleFunc("/healthz", h.handleHealth)

	// Canonical RESTful routes
	protected("/api/v1/objects", h.handleV1ObjectsRoot)
	protected("/api/v1/objects/", h.handleV1ObjectsPath)
	mux.HandleFunc("/api/v1/shares", h.handleShares)
	mux.HandleFunc("/api/v1/shares/", h.handleShares)
	mux.HandleFunc("/api/v1/public/objects/", h.handlePublicDownload)
	protected("/api/v1/admin/objects/", h.handleAdminObjects)

	// Direct S3-style routes
	protected("/objects/", h.handleObjects)
	protected("/admin/objects/", h.handleAdminObjects)

	// Legacy aliases for backward compatibility
	protected("/api/v1/upload-file", h.handleUploadAlias)
	protected("/api/v1/download-file", h.handleDownloadAlias)
	protected("/api/v1/delete-file", h.handleDeleteAlias)
	protected("/api/v1/hide-file", h.handleHideAlias)
	protected("/api/v1/admin/hide-file", h.handleAdminHideAlias)
	protected("/api/v1/admin/delete-file", h.handleAdminDeleteAlias)
	protected("/api/v1/get-user-files", h.handleGetUserFiles)
	protected("/api/v1/file-exists", h.handleFileExists)
	protected("/api/v1/make-private-read", h.permissionAlias("private-read"))
	protected("/api/v1/make-public-read", h.permissionAlias("public-read"))
	protected("/api/v1/share-link", h.handleCreateShareLink)
	mux.HandleFunc("/api/v1/share/download", h.handleDownloadByShareLink)
	mux.HandleFunc("/api/v1/public/download-file", h.handlePublicDownload)
}

func (h *HTTP) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *HTTP) handleObjects(w http.ResponseWriter, r *http.Request, p principal) {
	key := strings.TrimPrefix(r.URL.Path, "/objects/")
	if key == "" {
		respondError(w, http.StatusBadRequest, "object key is required")
		return
	}

	h.serveOwnerObject(w, r, p, key)
}

func (h *HTTP) handleV1ObjectsRoot(w http.ResponseWriter, r *http.Request, p principal) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetUserFiles(w, r, p)
	case http.MethodPost:
		h.handleUploadAlias(w, r, p)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTP) handleV1ObjectsPath(w http.ResponseWriter, r *http.Request, p principal) {
	key := strings.TrimPrefix(r.URL.Path, "/api/v1/objects/")
	if key == "" {
		h.handleV1ObjectsRoot(w, r, p)
		return
	}

	if objectKey, ok := strings.CutSuffix(key, "/permission"); ok {
		h.handleObjectPermission(w, r, p.UserID, objectKey)
		return
	}
	if objectKey, ok := strings.CutSuffix(key, "/shares"); ok {
		h.handleCreateShareLinkForKey(w, r, p.UserID, objectKey)
		return
	}
	if objectKey, ok := strings.CutSuffix(key, "/exists"); ok {
		h.objectExists(w, r, p.UserID, objectKey)
		return
	}
	h.serveOwnerObject(w, r, p, key)
}

func (h *HTTP) serveOwnerObject(w http.ResponseWriter, r *http.Request, p principal, key string) {
	if isVisibilityPath(key) {
		if p.Role == "" {
			p.Role = "user"
		}
		cleanKey := visibilityKey(key)
		h.handleObjectVisibility(w, r, p, p.UserID, cleanKey, "owner hide")
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.uploadObject(w, r, p.UserID, key)
	case http.MethodGet:
		h.downloadObject(w, r, p.UserID, key)
	case http.MethodHead:
		h.headObject(w, r, p.UserID, key)
	case http.MethodDelete:
		h.deleteObject(w, r, p.UserID, key)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func isVisibilityPath(path string) bool {
	return strings.HasSuffix(path, "/visibility") || strings.HasSuffix(path, "/hide")
}

func visibilityKey(path string) string {
	return strings.TrimSuffix(strings.TrimSuffix(path, "/visibility"), "/hide")
}
