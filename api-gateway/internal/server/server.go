package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"cloudbin-api-gateway/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

type Server struct {
	http *http.Server
}

var objectPathPrefixes = []string{
	"/objects/",
	"/admin/objects/",
}

var objectExactPaths = map[string]struct{}{
	"/api/v1/upload-file":          {},
	"/api/v1/download-file":        {},
	"/api/v1/delete-file":          {},
	"/api/v1/hide-file":            {},
	"/api/v1/admin/hide-file":      {},
	"/api/v1/admin/delete-file":    {},
	"/api/v1/get-user-files":       {},
	"/api/v1/make-private-read":    {},
	"/api/v1/make-public-read":     {},
	"/api/v1/public/download-file": {},
}

func New(cfg config.Config) *http.Server {
	authURL, err := url.Parse(cfg.AuthServiceURL)
	if err != nil {
		log.Fatalf("invalid AUTH_SERVICE_URL: %v", err)
	}
	objectURL, err := url.Parse(cfg.ObjectAPIURL)
	if err != nil {
		log.Fatalf("invalid OBJECT_API_URL: %v", err)
	}

	authProxy := httputil.NewSingleHostReverseProxy(authURL)
	objectProxy := httputil.NewSingleHostReverseProxy(objectURL)

	authRoutes := map[string]struct{}{
		"/api/v1/register":                   {},
		"/api/v1/register/verify-otp":        {},
		"/api/v1/login":                      {},
		"/api/v1/forgot-password":            {},
		"/api/v1/forgot-password/verify-otp": {},
		"/api/v1/owner-only":                 {},
		"/api/v1/create-token":               {},
		"/api/v1/list-tokens":                {},
		"/api/v1/delete-token":               {},
	}

	publicRoutes := map[string]struct{}{
		"/api/v1/register":                   {},
		"/api/v1/register/verify-otp":        {},
		"/api/v1/login":                      {},
		"/api/v1/forgot-password":            {},
		"/api/v1/forgot-password/verify-otp": {},
		"/api/v1/public/download-file":       {},
		"/healthz":                           {},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := publicRoutes[r.URL.Path]; !ok {
			if err := validateBearerJWT(r.Header.Get("Authorization"), cfg.JWTSecret, cfg.JWTIssuer); err != nil {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}

		if _, ok := authRoutes[r.URL.Path]; ok {
			authProxy.ServeHTTP(w, r)
			return
		}

		if isObjectPath(r.URL.Path) {
			objectProxy.ServeHTTP(w, r)
			return
		}

		respondJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
	})

	s := &Server{}
	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s.http
}

func isObjectPath(path string) bool {
	for _, prefix := range objectPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	if _, exists := objectExactPaths[path]; exists {
		return true
	}

	return false
}

func validateBearerJWT(authHeader, secret, issuer string) error {
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if token == "" {
		return errors.New("missing token")
	}

	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return err
	}
	if claims.Subject == "" || claims.Issuer != issuer {
		return errors.New("invalid claims")
	}
	return nil
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
