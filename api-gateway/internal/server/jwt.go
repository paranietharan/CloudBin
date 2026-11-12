package server

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	UserID string
	Role   string
	JTI    string
}

func validateBearerJWT(authHeader, secret, issuer string) (*TokenClaims, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
	if raw == "" {
		return nil, errors.New("missing token")
	}

	mapClaims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, mapClaims,
		func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		},
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	subject, _ := mapClaims["sub"].(string)
	if subject == "" {
		return nil, errors.New("missing subject claim")
	}

	role, _ := mapClaims["role"].(string)
	jti, _ := mapClaims["jti"].(string)

	return &TokenClaims{
		UserID: subject,
		Role:   role,
		JTI:    jti,
	}, nil
}
