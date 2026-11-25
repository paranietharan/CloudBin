package handler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type principal struct {
	UserID string
	Role   string
}

type authenticatedHandler func(http.ResponseWriter, *http.Request, principal)

func (h *HTTP) authMiddleware(next authenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Prefer verified headers from API Gateway if present
		if headerUserID := strings.TrimSpace(r.Header.Get("X-User-ID")); headerUserID != "" {
			role := strings.TrimSpace(r.Header.Get("X-User-Role"))
			if role == "" {
				role = "user"
			}
			next(w, r, principal{UserID: headerUserID, Role: role})
			return
		}

		auth := r.Header.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
		if token == "" {
			respondError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(h.jwtSecret), nil
		})
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		subject, _ := claims["sub"].(string)
		issuer, _ := claims["iss"].(string)
		role, _ := claims["role"].(string)
		if subject == "" || issuer != h.jwtIssuer {
			respondError(w, http.StatusUnauthorized, "invalid token issuer")
			return
		}
		next(w, r, principal{UserID: subject, Role: role})
	}
}

type windowCounter struct {
	windowStart time.Time
	count       int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]windowCounter
}

func (h *HTTP) allowByUserAndIP(r *http.Request, userID, action string, limit int, window time.Duration) bool {
	ip := "unknown"
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		ip = host
	}

	if h.redisClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		return h.allowRedis(ctx, fmt.Sprintf("ratelimit:user:%s:%s", userID, action), limit, window) &&
			h.allowRedis(ctx, fmt.Sprintf("ratelimit:ip:%s:%s", ip, action), limit, window)
	}

	return h.limiter.Allow(userID+":"+action, limit, window) && h.limiter.Allow(ip+":"+action, limit, window)
}

func (h *HTTP) allowRedis(ctx context.Context, key string, limit int, window time.Duration) bool {
	cnt, err := h.redisClient.Incr(ctx, key).Result()
	if err != nil {
		return true // Fail open on Redis error
	}
	if cnt == 1 {
		_ = h.redisClient.Expire(ctx, key, window).Err()
	}
	return cnt <= int64(limit)
}

func (l *fixedWindowLimiter) Allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// Periodic cleanup of stale entries to prevent memory leak
	if len(l.entries) > 1000 {
		for k, v := range l.entries {
			if now.Sub(v.windowStart) >= window*2 {
				delete(l.entries, k)
			}
		}
	}

	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.windowStart) >= window {
		l.entries[key] = windowCounter{windowStart: now, count: 1}
		return true
	}

	if entry.count >= limit {
		return false
	}

	entry.count++
	l.entries[key] = entry
	return true
}
