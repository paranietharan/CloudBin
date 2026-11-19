package server

import (
	"net/http"
	"testing"
)

func TestIsPublicRoute(t *testing.T) {
	tests := []struct {
		path   string
		method string
		want   bool
	}{
		{"/healthz", http.MethodGet, true},
		{"/api/v1/auth/register", http.MethodPost, true},
		{"/api/v1/auth/login", http.MethodPost, true},
		{"/api/v1/auth/tokens", http.MethodGet, false},
		{"/api/v1/shares/share-token-123", http.MethodGet, true},
		{"/api/v1/shares", http.MethodPost, false},
		{"/api/v1/public/objects/file.png", http.MethodGet, true},
		{"/api/v1/objects/file.png", http.MethodGet, false},
		{"/api/v1/register", http.MethodPost, true},
		{"/api/v1/share/download", http.MethodGet, true},
		{"/api/v1/public/download-file", http.MethodGet, true},
	}

	for _, tt := range tests {
		got := isPublicRoute(tt.path, tt.method)
		if got != tt.want {
			t.Errorf("isPublicRoute(%q, %q) = %v, want %v", tt.path, tt.method, got, tt.want)
		}
	}
}

func TestIsAuthPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v1/auth/register", true},
		{"/api/v1/auth/login", true},
		{"/api/v1/auth/tokens", true},
		{"/api/v1/auth/tokens/tok-1", true},
		{"/api/v1/tokens", true},
		{"/api/v1/tokens/tok-1", true},
		{"/api/v1/register", true},
		{"/api/v1/login", true},
		{"/api/v1/create-token", true},
		{"/api/v1/delete-token", true},
		{"/api/v1/objects", false},
		{"/healthz", false},
	}

	for _, tt := range tests {
		got := isAuthPath(tt.path)
		if got != tt.want {
			t.Errorf("isAuthPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsObjectPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/objects/my-file", true},
		{"/admin/objects/my-file", true},
		{"/api/v1/objects", true},
		{"/api/v1/objects/my-file", true},
		{"/api/v1/objects/my-file/permission", true},
		{"/api/v1/shares/my-token", true},
		{"/api/v1/public/objects/my-file", true},
		{"/api/v1/admin/objects/my-file", true},
		{"/api/v1/upload-file", true},
		{"/api/v1/download-file", true},
		{"/api/v1/get-user-files", true},
		{"/api/v1/auth/login", false},
		{"/healthz", false},
	}

	for _, tt := range tests {
		got := isObjectPath(tt.path)
		if got != tt.want {
			t.Errorf("isObjectPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
