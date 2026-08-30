package handler

import (
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/share/download", nil)
	w := httptest.NewRecorder()

	h.handleDownloadByShareLink(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
