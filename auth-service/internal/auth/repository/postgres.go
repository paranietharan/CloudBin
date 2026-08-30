package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"cloudbin-auth-service/internal/auth/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("conflict")

type Postgres struct {
	db *pgxpool.Pool
}

func NewPostgres(db *pgxpool.Pool) *Postgres {
	return &Postgres{db: db}
}

func (p *Postgres) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := p.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

func (p *Postgres) CreateUser(ctx context.Context, email, passwordHash, role string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := p.db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, role, is_verified, is_active, is_deleted, created_at, updated_at)
         VALUES ($1, $2, $3, $4, TRUE, TRUE, FALSE, NOW(), NOW())`,
		id, email, passwordHash, role,
	)
	if isUniqueViolation(err) {
		return uuid.Nil, ErrConflict
	}
	return id, err
}

func (p *Postgres) GetAuthUserByEmail(ctx context.Context, email string) (model.AuthUser, error) {
	var user model.AuthUser
	err := p.db.QueryRow(ctx,
		`SELECT id, email, password_hash, role, is_verified, is_active, is_deleted FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.IsVerified, &user.IsActive, &user.IsDeleted)
	return user, err
}

func (p *Postgres) UpdatePasswordByEmail(ctx context.Context, email, passwordHash string) (bool, error) {
	tag, err := p.db.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE email = $2 AND is_deleted = FALSE`,
		passwordHash, email,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (p *Postgres) InsertUserToken(ctx context.Context, userID uuid.UUID, jti, tokenName, tokenHash string, expiresAt time.Time) error {
	_, err := p.db.Exec(ctx,
		`INSERT INTO user_tokens (id, user_id, jti, token_name, token_hash, issued_at, expires_at, created_at)
         VALUES ($1, $2, $3, $4, $5, NOW(), $6, NOW())`,
		uuid.New(), userID, jti, tokenName, tokenHash, expiresAt,
	)
	return err
}

func (p *Postgres) ListUserTokens(ctx context.Context, userID string) ([]model.TokenInfo, error) {
	rows, err := p.db.Query(ctx,
		`SELECT id, token_name, issued_at, expires_at, revoked_at
         FROM user_tokens WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := make([]model.TokenInfo, 0)
	for rows.Next() {
		var (
			id        uuid.UUID
			name      *string
			issuedAt  time.Time
			expiresAt time.Time
			revokedAt *time.Time
		)
		if err := rows.Scan(&id, &name, &issuedAt, &expiresAt, &revokedAt); err != nil {
			return nil, err
		}
		tokenName := ""
		if name != nil {
			tokenName = *name
		}
		tokens = append(tokens, model.TokenInfo{
			ID:        id.String(),
			TokenName: tokenName,
			IssuedAt:  issuedAt,
			ExpiresAt: expiresAt,
			RevokedAt: revokedAt,
		})
	}
	return tokens, rows.Err()
}

func (p *Postgres) RevokeToken(ctx context.Context, userID, tokenID uuid.UUID) (string, time.Time, bool, error) {
	var jti string
	var expiresAt time.Time
	err := p.db.QueryRow(ctx,
		`UPDATE user_tokens SET revoked_at = NOW() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL RETURNING jti, expires_at`,
		tokenID, userID,
	).Scan(&jti, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, false, nil
	}
	if err != nil {
		return "", time.Time{}, false, err
	}
	return jti, expiresAt, true, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
