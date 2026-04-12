package server

import (
	"crypto/rand"
	"encoding/hex"
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
	"/api/v1/file-exists":          {},
	"/api/v1/make-private-read":    {},
	"/api/v1/make-public-read":     {},
	"/api/v1/share-link":           {},
	"/api/v1/share/download":       {},
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
		"/api/v1/share/download":             {},
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
				respondError(w, http.StatusUnauthorized, "unauthorized")
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

		respondError(w, http.StatusNotFound, "route not found")
	})

	s := &Server{}
	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      requestLoggingMiddleware(mux),
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%d", requestID, r.Method, r.URL.Path, rec.status, time.Since(start).Milliseconds())
	})
}

func generateRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "req-fallback"
	}
	return "req-" + hex.EncodeToString(buf)
}
