package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"cloudbin-auth-service/internal/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	cfg   config.Config
	db    *pgxpool.Pool
	redis *redis.Client
	http  *http.Server
}

type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func New(cfg config.Config) (*http.Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("connect auth db: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping auth db: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	s := &Server{cfg: cfg, db: db, redis: rdb}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/register", s.handleRegister)
	mux.HandleFunc("/api/v1/register/verify-otp", s.handleVerifyRegisterOTP)
	mux.HandleFunc("/api/v1/login", s.handleLogin)
	mux.HandleFunc("/api/v1/forgot-password", s.handleForgotPassword)
	mux.HandleFunc("/api/v1/forgot-password/verify-otp", s.handleVerifyForgotPasswordOTP)
	mux.HandleFunc("/api/v1/create-token", s.authMiddleware(s.handleCreateToken))
	mux.HandleFunc("/api/v1/list-tokens", s.authMiddleware(s.handleListTokens))
	mux.HandleFunc("/api/v1/delete-token", s.authMiddleware(s.handleDeleteToken))

	s.http = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s.http, nil
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
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

type otpPayload struct {
	Purpose      string `json:"purpose"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash,omitempty"`
	OTP          string `json:"otp"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 6 {
		respondError(w, http.StatusBadRequest, "email and password(min 6 chars) are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to validate user")
		return
	}
	if exists {
		respondError(w, http.StatusConflict, "email already exists")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	otp, err := generateOTP()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate otp")
		return
	}
	tempToken := uuid.NewString()

	payload := otpPayload{
		Purpose:      "register",
		Email:        req.Email,
		PasswordHash: string(hash),
		OTP:          otp,
	}
	if err := s.storeOTPPayload(ctx, tempToken, payload); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store otp state")
		return
	}

	log.Printf("register otp for %s: %s", req.Email, otp)

	respondJSON(w, http.StatusAccepted, map[string]any{
		"message":    "otp sent for registration",
		"temp_token": tempToken,
	})
}

func (s *Server) handleVerifyRegisterOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req verifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.TempToken = strings.TrimSpace(req.TempToken)
	req.OTP = strings.TrimSpace(req.OTP)
	if req.TempToken == "" || req.OTP == "" {
		respondError(w, http.StatusBadRequest, "temp_token and otp are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	payload, err := s.getOTPPayload(ctx, req.TempToken)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			respondError(w, http.StatusBadRequest, "otp expired or invalid temp token")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to read otp state")
		return
	}
	if payload.Purpose != "register" {
		respondError(w, http.StatusBadRequest, "invalid otp purpose")
		return
	}
	if payload.OTP != req.OTP {
		respondError(w, http.StatusUnauthorized, "invalid otp")
		return
	}

	id := uuid.New()
	_, err = s.db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, role, is_verified, is_active, is_deleted, created_at, updated_at)
         VALUES ($1, $2, $3, 'user', TRUE, TRUE, FALSE, NOW(), NOW())`,
		id, payload.Email, payload.PasswordHash,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			respondError(w, http.StatusConflict, "email already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	_ = s.redis.Del(ctx, s.otpRedisKey(req.TempToken)).Err()

	respondJSON(w, http.StatusCreated, map[string]any{
		"message": "user created",
		"user_id": id.String(),
	})
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "email is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND is_deleted = FALSE)`, req.Email).Scan(&exists)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to validate user")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "email not found")
		return
	}

	otp, err := generateOTP()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate otp")
		return
	}
	tempToken := uuid.NewString()

	payload := otpPayload{
		Purpose: "forgot_password",
		Email:   req.Email,
		OTP:     otp,
	}
	if err := s.storeOTPPayload(ctx, tempToken, payload); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store otp state")
		return
	}

	log.Printf("forgot-password otp for %s: %s", req.Email, otp)

	respondJSON(w, http.StatusOK, map[string]any{
		"message":    "otp sent for password reset",
		"temp_token": tempToken,
	})
}

func (s *Server) handleVerifyForgotPasswordOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req forgotPasswordVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.TempToken = strings.TrimSpace(req.TempToken)
	req.OTP = strings.TrimSpace(req.OTP)
	if req.TempToken == "" || req.OTP == "" || len(req.NewPassword) < 6 {
		respondError(w, http.StatusBadRequest, "temp_token, otp and new_password(min 6 chars) are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	payload, err := s.getOTPPayload(ctx, req.TempToken)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			respondError(w, http.StatusBadRequest, "otp expired or invalid temp token")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to read otp state")
		return
	}
	if payload.Purpose != "forgot_password" {
		respondError(w, http.StatusBadRequest, "invalid otp purpose")
		return
	}
	if payload.OTP != req.OTP {
		respondError(w, http.StatusUnauthorized, "invalid otp")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	tag, err := s.db.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE email = $2 AND is_deleted = FALSE`,
		string(hash), payload.Email,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update password")
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	_ = s.redis.Del(ctx, s.otpRedisKey(req.TempToken)).Err()

	respondJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var (
		userID       uuid.UUID
		passwordHash string
		role         string
		isVerified   bool
		isActive     bool
		isDeleted    bool
	)
	err := s.db.QueryRow(ctx,
		`SELECT id, password_hash, role, is_verified, is_active, is_deleted FROM users WHERE email = $1`,
		req.Email,
	).Scan(&userID, &passwordHash, &role, &isVerified, &isActive, &isDeleted)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if !isVerified {
		respondError(w, http.StatusForbidden, "user is not verified")
		return
	}
	if !isActive || isDeleted {
		respondError(w, http.StatusForbidden, "user is inactive or deleted")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	tokenID := uuid.NewString()
	token, tokenHash, exp, err := s.issueToken(userID.String(), role, tokenID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO user_tokens (id, user_id, jti, token_name, token_hash, issued_at, expires_at, created_at)
         VALUES ($1, $2, $3, $4, $5, NOW(), $6, NOW())`,
		uuid.New(), userID, tokenID, "login-session", tokenHash, exp,
	)
	if err != nil {
		log.Printf("warning: token metadata not stored: %v", err)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"token": token,
	})
}

type createTokenRequest struct {
	TokenName string `json:"token_name"`
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	principal, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.TokenName = strings.TrimSpace(req.TokenName)
	if req.TokenName == "" {
		req.TokenName = "service-token"
	}

	tokenID := uuid.NewString()
	token, tokenHash, exp, err := s.issueToken(principal.UserID, principal.Role, tokenID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	_, err = s.db.Exec(ctx,
		`INSERT INTO user_tokens (id, user_id, jti, token_name, token_hash, issued_at, expires_at, created_at)
         VALUES ($1, $2, $3, $4, $5, NOW(), $6, NOW())`,
		uuid.New(), principal.UserID, tokenID, req.TokenName, tokenHash, exp,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store token")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"token_name": req.TokenName,
		"expires_at": exp,
	})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	principal, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := s.db.Query(ctx,
		`SELECT id, token_name, issued_at, expires_at, revoked_at
         FROM user_tokens WHERE user_id = $1 ORDER BY created_at DESC`,
		principal.UserID,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	defer rows.Close()

	type tokenInfo struct {
		ID        string     `json:"id"`
		TokenName string     `json:"token_name"`
		IssuedAt  time.Time  `json:"issued_at"`
		ExpiresAt time.Time  `json:"expires_at"`
		RevokedAt *time.Time `json:"revoked_at,omitempty"`
	}
	tokens := make([]tokenInfo, 0)
	for rows.Next() {
		var (
			id        uuid.UUID
			name      *string
			issuedAt  time.Time
			expiresAt time.Time
			revokedAt *time.Time
		)
		if err := rows.Scan(&id, &name, &issuedAt, &expiresAt, &revokedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to parse tokens")
			return
		}
		n := ""
		if name != nil {
			n = *name
		}
		tokens = append(tokens, tokenInfo{
			ID:        id.String(),
			TokenName: n,
			IssuedAt:  issuedAt,
			ExpiresAt: expiresAt,
			RevokedAt: revokedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

type deleteTokenRequest struct {
	TokenID string `json:"token_id"`
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	principal, ok := principalFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req deleteTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(req.TokenID))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid token_id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tag, err := s.db.Exec(ctx,
		`UPDATE user_tokens SET revoked_at = NOW() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		id, principal.UserID,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete token")
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, http.StatusNotFound, "token not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "token revoked"})
}

func (s *Server) issueToken(userID, role, tokenID string) (string, string, time.Time, error) {
	ttl, err := time.ParseDuration(s.cfg.JWTAccessTTL)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("parse JWT_ACCESS_TTL: %w", err)
	}
	now := time.Now().UTC()
	exp := now.Add(ttl)

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.cfg.JWTIssuer,
			ID:        tokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	})

	signed, err := t.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", "", time.Time{}, err
	}

	sum := sha256.Sum256([]byte(signed))
	return signed, hex.EncodeToString(sum[:]), exp, nil
}

type principal struct {
	UserID string
	Role   string
}

type principalContextKey struct{}

func principalFromContext(ctx context.Context) (principal, bool) {
	v := ctx.Value(principalContextKey{})
	if v == nil {
		return principal{}, false
	}
	p, ok := v.(principal)
	return p, ok
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
		if token == "" {
			respondError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		c := &claims{}
		_, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(s.cfg.JWTSecret), nil
		})
		if err != nil || c.Subject == "" {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		if c.Issuer != s.cfg.JWTIssuer {
			respondError(w, http.StatusUnauthorized, "invalid token issuer")
			return
		}

		ctx := context.WithValue(r.Context(), principalContextKey{}, principal{UserID: c.Subject, Role: c.Role})
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) otpRedisKey(tempToken string) string {
	return "auth:otp:" + tempToken
}

func (s *Server) otpTTL() time.Duration {
	ttl, err := time.ParseDuration(s.cfg.OTPTTL)
	if err != nil || ttl <= 0 {
		return 5 * time.Minute
	}
	return ttl
}

func (s *Server) storeOTPPayload(ctx context.Context, tempToken string, payload otpPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, s.otpRedisKey(tempToken), raw, s.otpTTL()).Err()
}

func (s *Server) getOTPPayload(ctx context.Context, tempToken string) (otpPayload, error) {
	raw, err := s.redis.Get(ctx, s.otpRedisKey(tempToken)).Result()
	if err != nil {
		return otpPayload{}, err
	}
	var payload otpPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return otpPayload{}, err
	}
	return payload, nil
}

func generateOTP() (string, error) {
	b := make([]byte, 6)
	for i := 0; i < len(b); i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b[i] = byte('0' + n.Int64())
	}
	return string(b), nil
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
