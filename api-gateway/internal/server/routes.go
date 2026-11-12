package server

import "net/http"

// Requests under these prefixes are routed to the object-api.
var objectPathPrefixes = []string{
	"/objects/",
	"/admin/objects/",
	"/api/v1/objects",
	"/api/v1/shares",
	"/api/v1/public/",
	"/api/v1/admin/objects",
}

var objectExactPaths = map[string]struct{}{
	"/api/v1/upload-file":          {},
	"/api/v1/download-file":        {},
	"/api/v1/delete-file":          {},
	"/api/v1/hide-file":            {},
	"/api/v1/admin/hide-file":      {},
	"/api/v1/admin/delete-file":    {},
	"/api/v1/get-user-files":       {},
	"/api/v1/file-exists":          {},
	"/api/v1/make-private-read":    {},
	"/api/v1/make-public-read":     {},
	"/api/v1/share-link":           {},
	"/api/v1/share/download":       {},
	"/api/v1/public/download-file": {},
}

// Requests under these prefixes are routed to the auth-service.
var authPathPrefixes = []string{
	"/api/v1/auth/",
	"/api/v1/tokens/",
}

var authExactRoutes = map[string]struct{}{
	"/api/v1/register":                   {},
	"/api/v1/register/verify-otp":        {},
	"/api/v1/login":                      {},
	"/api/v1/forgot-password":            {},
	"/api/v1/forgot-password/verify-otp": {},
	"/api/v1/owner-only":                 {},
	"/api/v1/create-token":               {},
	"/api/v1/list-tokens":                {},
	"/api/v1/delete-token":               {},
	"/api/v1/tokens":                     {},
}

// publicExactRoutes are always allowed without a JWT token.
var publicExactRoutes = map[string]struct{}{
	"/healthz":                                {},
	"/api/v1/auth/register":                   {},
	"/api/v1/auth/register/verify":            {},
	"/api/v1/auth/register/verify-otp":        {},
	"/api/v1/auth/login":                      {},
	"/api/v1/auth/forgot-password":            {},
	"/api/v1/auth/forgot-password/verify":     {},
	"/api/v1/auth/forgot-password/verify-otp": {},
	"/api/v1/register":                        {},
	"/api/v1/register/verify-otp":             {},
	"/api/v1/login":                           {},
	"/api/v1/forgot-password":                 {},
	"/api/v1/forgot-password/verify-otp":      {},
	"/api/v1/share/download":                  {},
	"/api/v1/public/download-file":            {},
}

func isPublicRoute(path, method string) bool {
	if _, ok := publicExactRoutes[path]; ok {
		return true
	}
	// GET-only public prefixes (share links and public objects).
	if method == http.MethodGet {
		return hasAnyPrefix(path, "/api/v1/shares/", "/api/v1/public/")
	}
	return false
}

func isAuthPath(path string) bool {
	if hasAnyPrefix(path, authPathPrefixes...) {
		return true
	}
	_, ok := authExactRoutes[path]
	return ok
}

func isObjectPath(path string) bool {
	if hasAnyPrefix(path, objectPathPrefixes...) {
		return true
	}
	_, ok := objectExactPaths[path]
	return ok
}

// hasAnyPrefix reports whether s starts with any of the given prefixes.
func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if len(s) >= len(p) && s[:len(p)] == p {
			return true
		}
	}
	return false
}
