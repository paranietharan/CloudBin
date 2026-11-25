package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"cloudbin-auth-service/internal/auth/service"
	"cloudbin-shared/response"

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

type authenticatedHandler func(http.ResponseWriter, *http.Request, principal)

func NewHTTP(svc *service.Service, jwtSecret, jwtIssuer string) *HTTP {
	return &HTTP{svc: svc, jwtSecret: jwtSecret, jwtIssuer: jwtIssuer}
}

func (h *HTTP) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", h.handleHealth)

	// Canonical RESTful routes
	mux.HandleFunc("/api/v1/auth/register", h.handleRegister)
	mux.HandleFunc("/api/v1/auth/register/verify", h.handleVerifyRegisterOTP)
	mux.HandleFunc("/api/v1/auth/register/verify-otp", h.handleVerifyRegisterOTP)
	mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("/api/v1/auth/forgot-password", h.handleForgotPassword)
	mux.HandleFunc("/api/v1/auth/forgot-password/verify", h.handleVerifyForgotPasswordOTP)
	mux.HandleFunc("/api/v1/auth/forgot-password/verify-otp", h.handleVerifyForgotPasswordOTP)
	mux.HandleFunc("/api/v1/auth/tokens", h.authMiddleware(h.handleTokens))
	mux.HandleFunc("/api/v1/auth/tokens/", h.authMiddleware(h.handleDeleteToken))

	// Direct tokens resource aliases
	mux.HandleFunc("/api/v1/tokens", h.authMiddleware(h.handleTokens))
	mux.HandleFunc("/api/v1/tokens/", h.authMiddleware(h.handleDeleteToken))

	// Legacy aliases for backward compatibility
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
	response.JSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type credentialsRequest struct {
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

type createTokenRequest struct {
	TokenName string `json:"token_name"`
}

type deleteTokenRequest struct {
	TokenID string `json:"token_id"`
}

func (h *HTTP) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tempToken, err := h.svc.BeginRegistration(ctx, req.Email, req.Password)
	if err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]any{"message": "otp sent for registration", "temp_token": tempToken})
}

func (h *HTTP) handleVerifyRegisterOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req verifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id, err := h.svc.VerifyRegistrationOTP(ctx, strings.TrimSpace(req.TempToken), strings.TrimSpace(req.OTP))
	if err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"message": "user created", "user_id": id.String()})
}

func (h *HTTP) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tempToken, err := h.svc.BeginForgotPassword(ctx, req.Email)
	if err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"message": "otp sent for password reset", "temp_token": tempToken})
}

func (h *HTTP) handleVerifyForgotPasswordOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req forgotPasswordVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := h.svc.VerifyForgotPasswordOTP(ctx, strings.TrimSpace(req.TempToken), strings.TrimSpace(req.OTP), req.NewPassword)
	if err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"message": "password updated"})
}

func (h *HTTP) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token, userID, err := h.svc.Login(ctx, req.Email, req.Password)
	if err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"token": token, "user_id": userID})
}

func (h *HTTP) handleOwnerOnly(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	if !isOwnerOrAdmin(p, ownerID) {
		response.Error(w, http.StatusForbidden, "only owner or admin can access this resource")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{"message": "hello"})
}

func (h *HTTP) handleCreateToken(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token, exp, err := h.svc.CreateToken(ctx, p.UserID, p.Role, req.TokenName)
	if err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{"token": token, "token_name": req.TokenName, "expires_at": exp})
}

func (h *HTTP) handleListTokens(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tokens, err := h.svc.ListTokens(ctx, p.UserID)
	if err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (h *HTTP) handleTokens(w http.ResponseWriter, r *http.Request, p principal) {
	switch r.Method {
	case http.MethodGet:
		h.handleListTokens(w, r, p)
	case http.MethodPost:
		h.handleCreateToken(w, r, p)
	default:
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTP) handleDeleteToken(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodDelete {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	tokenID := readTokenID(r)
	if tokenID == "" {
		response.Error(w, http.StatusBadRequest, "token_id is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.svc.DeleteToken(ctx, p.UserID, tokenID); err != nil {
		h.mapError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"message": "token revoked"})
}

func readTokenID(r *http.Request) string {
	path := r.URL.Path
	for _, prefix := range []string{"/api/v1/auth/tokens/", "/api/v1/tokens/"} {
		if strings.HasPrefix(path, prefix) {
			id := strings.Trim(strings.TrimPrefix(path, prefix), "/")
			if id != "" {
				return id
			}
		}
	}
	if q := strings.TrimSpace(r.URL.Query().Get("token_id")); q != "" {
		return q
	}
	if r.Body != nil && r.ContentLength > 0 {
		var req deleteTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			return strings.TrimSpace(req.TokenID)
		}
	}
	return ""
}

func (h *HTTP) authMiddleware(next authenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
		if token == "" {
			response.Error(w, http.StatusUnauthorized, "missing bearer token")
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
			response.Error(w, http.StatusUnauthorized, "invalid token")
			return
		}
		subject, _ := claims["sub"].(string)
		issuer, _ := claims["iss"].(string)
		role, _ := claims["role"].(string)
		jti, _ := claims["jti"].(string)
		if subject == "" || issuer != h.jwtIssuer {
			response.Error(w, http.StatusUnauthorized, "invalid token issuer")
			return
		}
		if jti != "" && h.svc.IsTokenRevoked(r.Context(), jti) {
			response.Error(w, http.StatusUnauthorized, "token has been revoked")
			return
		}
		next(w, r, principal{UserID: subject, Role: role})
	}
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
		response.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrEmailNotFound), errors.Is(err, service.ErrTokenNotFound):
		response.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, service.ErrInvalidOTP):
		response.Error(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrNotVerified), errors.Is(err, service.ErrInactiveOrDeleted):
		response.Error(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrOTPExpired), errors.Is(err, service.ErrInvalidPurpose):
		response.Error(w, http.StatusBadRequest, err.Error())
	default:
		if strings.Contains(err.Error(), "required") {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "internal server error")
	}
}
