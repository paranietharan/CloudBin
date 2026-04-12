package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"cloudbin-auth-service/internal/auth/service"

	"github.com/golang-jwt/jwt/v5"
)

type HTTP struct {
	svc       *service.Service
	jwtSecret string
	jwtIssuer string
}

type principal struct {
	UserID string
	Role   string
}

type principalContextKey struct{}

func NewHTTP(svc *service.Service, jwtSecret, jwtIssuer string) *HTTP {
	return &HTTP{svc: svc, jwtSecret: jwtSecret, jwtIssuer: jwtIssuer}
}

func (h *HTTP) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/api/v1/register", h.handleRegister)
	mux.HandleFunc("/api/v1/register/verify-otp", h.handleVerifyRegisterOTP)
	mux.HandleFunc("/api/v1/login", h.handleLogin)
	mux.HandleFunc("/api/v1/forgot-password", h.handleForgotPassword)
	mux.HandleFunc("/api/v1/forgot-password/verify-otp", h.handleVerifyForgotPasswordOTP)
	mux.HandleFunc("/api/v1/owner-only", h.authMiddleware(h.handleOwnerOnly))
	mux.HandleFunc("/api/v1/create-token", h.authMiddleware(h.handleCreateToken))
	mux.HandleFunc("/api/v1/list-tokens", h.authMiddleware(h.handleListTokens))
	mux.HandleFunc("/api/v1/delete-token", h.authMiddleware(h.handleDeleteToken))
}

func (h *HTTP) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type verifyOTPRequest struct {
	TempToken string `json:"temp_token"`
	OTP       string `json:"otp"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type forgotPasswordVerifyRequest struct {
	TempToken   string `json:"temp_token"`
	OTP         string `json:"otp"`
	NewPassword string `json:"new_password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createTokenRequest struct {
	TokenName string `json:"token_name"`
}

type deleteTokenRequest struct {
	TokenID string `json:"token_id"`
}

func (h *HTTP) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tempToken, err := h.svc.BeginRegistration(ctx, req.Email, req.Password)
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"message": "otp sent for registration", "temp_token": tempToken})
}

func (h *HTTP) handleVerifyRegisterOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req verifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, err := h.svc.VerifyRegistrationOTP(ctx, strings.TrimSpace(req.TempToken), strings.TrimSpace(req.OTP))
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"message": "user created", "user_id": id.String()})
}

func (h *HTTP) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tempToken, err := h.svc.BeginForgotPassword(ctx, req.Email)
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"message": "otp sent for password reset", "temp_token": tempToken})
}

func (h *HTTP) handleVerifyForgotPasswordOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req forgotPasswordVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := h.svc.VerifyForgotPasswordOTP(ctx, strings.TrimSpace(req.TempToken), strings.TrimSpace(req.OTP), req.NewPassword)
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

func (h *HTTP) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token, userID, err := h.svc.Login(ctx, req.Email, req.Password)
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"token": token, "user_id": userID})
}

func (h *HTTP) handleOwnerOnly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if !isOwnerOrAdmin(p, ownerID) {
		respondError(w, http.StatusForbidden, "only owner or admin can access this resource")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "hello"})
}

func (h *HTTP) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token, exp, err := h.svc.CreateToken(ctx, p.UserID, p.Role, req.TokenName)
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"token": token, "token_name": req.TokenName, "expires_at": exp})
}

func (h *HTTP) handleListTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tokens, err := h.svc.ListTokens(ctx, p.UserID)
	if err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (h *HTTP) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req deleteTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.svc.DeleteToken(ctx, p.UserID, req.TokenID); err != nil {
		h.mapError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "token revoked"})
}

func (h *HTTP) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		next(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal{UserID: subject, Role: role})))
	}
}

func principalFromContext(ctx context.Context) (principal, bool) {
	v := ctx.Value(principalContextKey{})
	if v == nil {
		return principal{}, false
	}
	p, ok := v.(principal)
	return p, ok
}

func isOwnerOrAdmin(p principal, ownerID string) bool {
	if p.Role == "admin" || p.Role == "owner" {
		return true
	}
	return ownerID != "" && p.UserID == ownerID
}

func (h *HTTP) mapError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, service.ErrEmailExists):
		respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrEmailNotFound), errors.Is(err, service.ErrTokenNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, service.ErrInvalidOTP):
		respondError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrNotVerified), errors.Is(err, service.ErrInactiveOrDeleted):
		respondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrOTPExpired), errors.Is(err, service.ErrInvalidPurpose):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		if strings.Contains(err.Error(), "required") {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
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
