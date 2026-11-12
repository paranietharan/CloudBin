package server

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware generates (or propagates) a request ID, sets it on both
// the outgoing request and the response, then logs the result after the
// handler returns.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if reqID == "" {
			reqID = newRequestID()
		}
		r.Header.Set("X-Request-ID", reqID) // forward to backend services
		w.Header().Set("X-Request-ID", reqID)

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%d",
			reqID, r.Method, r.URL.Path, rec.status, time.Since(start).Milliseconds())
	})
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "req-fallback"
	}
	return "req-" + hex.EncodeToString(buf)
}
