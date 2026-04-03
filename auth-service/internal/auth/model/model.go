package model

import (
	"time"

	"github.com/google/uuid"
)

type AuthUser struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Role         string
	IsVerified   bool
	IsActive     bool
	IsDeleted    bool
}

type TokenInfo struct {
	ID        string     `json:"id"`
	TokenName string     `json:"token_name"`
	IssuedAt  time.Time  `json:"issued_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type OTPPayload struct {
	Purpose      string `json:"purpose"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash,omitempty"`
	OTP          string `json:"otp"`
}
