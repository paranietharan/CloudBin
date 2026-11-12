package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"cloudbin-api-gateway/internal/config"
)

func New(cfg config.Config) *http.Server {
	authProxy := mustProxy(cfg.AuthServiceURL)
	objectProxy := mustProxy(cfg.ObjectAPIURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/", makeHandler(cfg, authProxy, objectProxy))

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

func makeHandler(cfg config.Config, authProxy, objectProxy http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-User-ID")
		r.Header.Del("X-User-Role")
		r.Header.Del("X-Token-JTI")

		if !isPublicRoute(r.URL.Path, r.Method) {
			claims, err := validateBearerJWT(r.Header.Get("Authorization"), cfg.JWTSecret, cfg.JWTIssuer)
			if err != nil {
				respondError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			r.Header.Set("X-User-ID", claims.UserID)
			r.Header.Set("X-User-Role", claims.Role)
			r.Header.Set("X-Token-JTI", claims.JTI)
		}

		switch {
		case isAuthPath(r.URL.Path):
			authProxy.ServeHTTP(w, r)
		case isObjectPath(r.URL.Path):
			objectProxy.ServeHTTP(w, r)
		default:
			respondError(w, http.StatusNotFound, "route not found")
		}
	}
}

func mustProxy(rawURL string) *httputil.ReverseProxy {
	target, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("invalid proxy target %q: %v", rawURL, err)
	}
	p := httputil.NewSingleHostReverseProxy(target)
	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error: %s %s → %v", r.Method, r.URL.Path, err)
		respondError(w, http.StatusBadGateway, "upstream service unavailable")
	}
	return p
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
