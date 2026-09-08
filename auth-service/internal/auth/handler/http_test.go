package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleRegisterValidation(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/register", nil)
	w := httptest.NewRecorder()

	h.handleRegister(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleLoginValidation(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	w := httptest.NewRecorder()

	h.handleLogin(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleTokensUnauthorized(t *testing.T) {
	h := NewHTTP(nil, "secret", "issuer")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	w := httptest.NewRecorder()

	handler := h.authMiddleware(h.handleTokens)
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestReadTokenID(t *testing.T) {
	// 1. From path
	req1 := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/tok-123", nil)
	if id := readTokenID(req1); id != "tok-123" {
		t.Fatalf("expected tok-123, got %s", id)
	}

	// 2. From short path
	req2 := httptest.NewRequest(http.MethodDelete, "/api/v1/tokens/tok-456", nil)
	if id := readTokenID(req2); id != "tok-456" {
		t.Fatalf("expected tok-456, got %s", id)
	}

	// 3. From query
	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/delete-token?token_id=tok-789", nil)
	if id := readTokenID(req3); id != "tok-789" {
		t.Fatalf("expected tok-789, got %s", id)
	}

	// 4. From body
	body := bytes.NewBufferString(`{"token_id": "tok-body"}`)
	req4 := httptest.NewRequest(http.MethodDelete, "/api/v1/delete-token", body)
	req4.Header.Set("Content-Type", "application/json")
	if id := readTokenID(req4); id != "tok-body" {
		t.Fatalf("expected tok-body, got %s", id)
	}
}
