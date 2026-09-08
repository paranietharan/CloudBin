package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCreateShareLinkMethodNotAllowed(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/share-link", nil)
	w := httptest.NewRecorder()

	h.handleCreateShareLink(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleDownloadByShareLinkMissingToken(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares/", nil)
	w := httptest.NewRecorder()

	h.handleDownloadByShareLink(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlePublicDownloadMissingParams(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/objects/my-file.txt", nil)
	w := httptest.NewRecorder()

	h.handlePublicDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleObjectPermissionMethodNotAllowed(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/objects/test/permission", nil)
	w := httptest.NewRecorder()

	h.handleObjectPermission(w, req, "user-1", "test")

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleObjectPermissionInvalidBody(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer", nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/objects/test/permission", nil)
	w := httptest.NewRecorder()

	h.handleObjectPermission(w, req, "user-1", "test")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleV1ObjectsPathUnauthorized(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/objects/test", nil)
	w := httptest.NewRecorder()

	h.handleV1ObjectsPath(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestReadObjectKey(t *testing.T) {
	// Query object_key
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/objects?object_key=file1.png", nil)
	if k := readObjectKey(req1); k != "file1.png" {
		t.Fatalf("expected file1.png, got %s", k)
	}

	// Query key
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/objects?key=file2.png", nil)
	if k := readObjectKey(req2); k != "file2.png" {
		t.Fatalf("expected file2.png, got %s", k)
	}

	// Header X-Object-Key
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/objects", nil)
	req3.Header.Set("X-Object-Key", "file3.png")
	if k := readObjectKey(req3); k != "file3.png" {
		t.Fatalf("expected file3.png, got %s", k)
	}
}

func TestHandleV1ObjectsPathRouting(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer", nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Context with principal
	pCtx := context.WithValue(context.Background(), principalContextKey{}, principal{UserID: "u1", Role: "user"})

	// Test GET /api/v1/objects/some-key/permission (Method Not Allowed)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/objects/some-key/permission", nil).WithContext(pCtx)
	w := httptest.NewRecorder()
	h.handleV1ObjectsPath(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}
