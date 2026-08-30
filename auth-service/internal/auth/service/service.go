package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"cloudbin-auth-service/internal/auth/model"
	"cloudbin-auth-service/internal/auth/otp"
	"cloudbin-auth-service/internal/auth/repository"
	"cloudbin-auth-service/internal/email"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrNotVerified        = errors.New("user is not verified")
	ErrInactiveOrDeleted  = errors.New("user is inactive or deleted")
	ErrInvalidOTP         = errors.New("invalid otp")
	ErrOTPExpired         = errors.New("otp expired or invalid temp token")
	ErrInvalidPurpose     = errors.New("invalid otp purpose")
	ErrEmailExists        = errors.New("email already exists")
	ErrEmailNotFound      = errors.New("email not found")
	ErrTokenNotFound      = errors.New("token not found")
)

type UserRepository interface {
	UserExistsByEmail(ctx context.Context, email string) (bool, error)
	CreateUser(ctx context.Context, email, passwordHash, role string) (uuid.UUID, error)
	GetAuthUserByEmail(ctx context.Context, email string) (model.AuthUser, error)
	UpdatePasswordByEmail(ctx context.Context, email, passwordHash string) (bool, error)
	InsertUserToken(ctx context.Context, userID uuid.UUID, jti, tokenName, tokenHash string, expiresAt time.Time) error
	ListUserTokens(ctx context.Context, userID string) ([]model.TokenInfo, error)
	RevokeToken(ctx context.Context, userID, tokenID uuid.UUID) (string, time.Time, bool, error)
}

type OTPStore interface {
	Save(ctx context.Context, tempToken string, payload model.OTPPayload) error
	Get(ctx context.Context, tempToken string) (model.OTPPayload, error)
	Delete(ctx context.Context, tempToken string) error
}

type Service struct {
	repo         UserRepository
	otpStore     OTPStore
	redisClient  *redis.Client
	jwtSecret    string
	jwtIssuer    string
	jwtTTL       time.Duration
	devMode      bool
	emailService *email.EmailService
}

func New(repo UserRepository, otpStore *otp.Store, redisClient *redis.Client, jwtSecret, jwtIssuer string, jwtTTL time.Duration, devMode bool, smtpHost string, smtpPort int, smtpUser, smtpPass, smtpFromEmail, smtpFromName string) *Service {
	if jwtTTL <= 0 {
		jwtTTL = 24 * time.Hour
	}
	return &Service{
		repo:         repo,
		otpStore:     otpStore,
		redisClient:  redisClient,
		jwtSecret:    jwtSecret,
		jwtIssuer:    jwtIssuer,
		jwtTTL:       jwtTTL,
		devMode:      devMode,
		emailService: email.NewEmailService(smtpHost, smtpPort, smtpUser, smtpPass, smtpFromEmail, smtpFromName),
	}
}

func (s *Service) BeginRegistration(ctx context.Context, email, password string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || len(password) < 6 {
		return "", errors.New("email and password(min 6 chars) are required")
	}

	exists, err := s.repo.UserExistsByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if exists {
		return "", ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	otpCode, err := generateOTP()
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	tempToken := uuid.NewString()

	payload := model.OTPPayload{Purpose: "register", Email: email, PasswordHash: string(hash), OTP: otpCode}
	if err := s.otpStore.Save(ctx, tempToken, payload); err != nil {
		return "", err
	}

	emailSent := false
	if s.emailService != nil && s.emailService.IsConfigured() {
		if err := s.emailService.SendAccountCreationOTPEmail(email, otpCode); err != nil {
			log.Printf("warning: failed to send registration OTP email to %s: %v", email, err)
		} else {
			emailSent = true
		}
	}

	if s.devMode || !emailSent {
		log.Printf("register otp for %s: %s (email_sent=%t)", email, otpCode, emailSent)
	} else {
		log.Printf("register otp sent for %s", email)
	}

	return tempToken, nil
}

func (s *Service) VerifyRegistrationOTP(ctx context.Context, tempToken, otpCode string) (uuid.UUID, error) {
	payload, err := s.otpStore.Get(ctx, tempToken)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil, ErrOTPExpired
		}
		return uuid.Nil, err
	}
	if payload.Purpose != "register" {
		return uuid.Nil, ErrInvalidPurpose
	}
	if strings.TrimSpace(payload.OTP) != strings.TrimSpace(otpCode) {
		return uuid.Nil, ErrInvalidOTP
	}

	id, err := s.repo.CreateUser(ctx, payload.Email, payload.PasswordHash, "user")
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return uuid.Nil, ErrEmailExists
		}
		return uuid.Nil, err
	}
	_ = s.otpStore.Delete(ctx, tempToken)

	if s.emailService != nil && s.emailService.IsConfigured() {
		if err := s.emailService.SendAccountCreatedEmail(payload.Email); err != nil {
			log.Printf("warning: failed to send account created email to %s: %v", payload.Email, err)
		}
	}

	return id, nil
}

func (s *Service) BeginForgotPassword(ctx context.Context, email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", errors.New("email is required")
	}

	exists, err := s.repo.UserExistsByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrEmailNotFound
	}

	otpCode, err := generateOTP()
	if err != nil {
		return "", err
	}
	tempToken := uuid.NewString()
	payload := model.OTPPayload{Purpose: "forgot_password", Email: email, OTP: otpCode}
	if err := s.otpStore.Save(ctx, tempToken, payload); err != nil {
		return "", err
	}

	emailSent := false
	if s.emailService != nil && s.emailService.IsConfigured() {
		if err := s.emailService.SendForgotPasswordEmail(email, otpCode); err != nil {
			log.Printf("warning: failed to send forgot-password OTP email to %s: %v", email, err)
		} else {
			emailSent = true
		}
	}

	if s.devMode || !emailSent {
		log.Printf("forgot-password otp for %s: %s (email_sent=%t)", email, otpCode, emailSent)
	} else {
		log.Printf("forgot-password otp sent for %s", email)
	}

	return tempToken, nil
}

func (s *Service) VerifyForgotPasswordOTP(ctx context.Context, tempToken, otpCode, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("new_password(min 6 chars) is required")
	}

	payload, err := s.otpStore.Get(ctx, tempToken)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrOTPExpired
		}
		return err
	}
	if payload.Purpose != "forgot_password" {
		return ErrInvalidPurpose
	}
	if strings.TrimSpace(payload.OTP) != strings.TrimSpace(otpCode) {
		return ErrInvalidOTP
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	updated, err := s.repo.UpdatePasswordByEmail(ctx, payload.Email, string(hash))
	if err != nil {
		return err
	}
	if !updated {
		return ErrEmailNotFound
	}

	_ = s.otpStore.Delete(ctx, tempToken)
	return nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.repo.GetAuthUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrInvalidCredentials
		}
		return "", "", err
	}
	if !user.IsVerified {
		return "", "", ErrNotVerified
	}
	if !user.IsActive || user.IsDeleted {
		return "", "", ErrInactiveOrDeleted
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}

	tokenID := uuid.NewString()
	token, tokenHash, exp, err := s.issueToken(user.ID.String(), user.Role, tokenID)
	if err != nil {
		return "", "", err
	}
	if err := s.repo.InsertUserToken(ctx, user.ID, tokenID, "login-session", tokenHash, exp); err != nil {
		log.Printf("warning: token metadata not stored: %v", err)
	}
	return token, user.ID.String(), nil
}

func (s *Service) CreateToken(ctx context.Context, userID, role, tokenName string) (string, time.Time, error) {
	if strings.TrimSpace(tokenName) == "" {
		tokenName = "service-token"
	}

	tokenID := uuid.NewString()
	token, tokenHash, exp, err := s.issueToken(userID, role, tokenID)
	if err != nil {
		return "", time.Time{}, err
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", time.Time{}, err
	}

	if err := s.repo.InsertUserToken(ctx, uid, tokenID, tokenName, tokenHash, exp); err != nil {
		return "", time.Time{}, err
	}
	return token, exp, nil
}

func (s *Service) ListTokens(ctx context.Context, userID string) ([]model.TokenInfo, error) {
	return s.repo.ListUserTokens(ctx, userID)
}

func (s *Service) DeleteToken(ctx context.Context, userID, tokenID string) error {
	uid, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	tid, err := uuid.Parse(strings.TrimSpace(tokenID))
	if err != nil {
		return err
	}

	jti, exp, ok, err := s.repo.RevokeToken(ctx, uid, tid)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTokenNotFound
	}

	if s.redisClient != nil && jti != "" {
		ttl := time.Until(exp)
		if ttl <= 0 {
			ttl = time.Minute
		}
		_ = s.redisClient.Set(ctx, "auth:revoked:"+jti, "1", ttl).Err()
	}

	return nil
}

func (s *Service) IsTokenRevoked(ctx context.Context, jti string) bool {
	if s.redisClient == nil || jti == "" {
		return false
	}
	val, err := s.redisClient.Exists(ctx, "auth:revoked:"+jti).Result()
	if err != nil {
		return false
	}
	return val > 0
}

func (s *Service) issueToken(userID, role, tokenID string) (string, string, time.Time, error) {
	now := time.Now().UTC()
	exp := now.Add(s.jwtTTL)

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"iss":  s.jwtIssuer,
		"jti":  tokenID,
		"role": role,
		"iat":  now.Unix(),
		"exp":  exp.Unix(),
	})

	signed, err := t.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", time.Time{}, err
	}

	sum := sha256.Sum256([]byte(signed))
	return signed, hex.EncodeToString(sum[:]), exp, nil
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
