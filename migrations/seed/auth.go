package seed

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultAdminEmail = "admin@example.com"
	defaultUserEmail  = "user@example.com"
)

func RunAuth() error {
	dsn := strings.TrimSpace(os.Getenv("AUTH_DB_DSN"))
	if dsn == "" {
		return errors.New("AUTH_DB_DSN is required")
	}

	adminPassword := strings.TrimSpace(os.Getenv("ADMIN_INITIAL_PASSWORD"))
	if adminPassword == "" {
		return errors.New("ADMIN_INITIAL_PASSWORD is required")
	}

	userPassword := strings.TrimSpace(os.Getenv("USER_INITIAL_PASSWORD"))
	if userPassword == "" {
		userPassword = adminPassword
	}

	adminEmail := strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))
	if adminEmail == "" {
		adminEmail = defaultAdminEmail
	}

	userEmail := strings.TrimSpace(os.Getenv("USER_EMAIL"))
	if userEmail == "" {
		userEmail = defaultUserEmail
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect auth db: %w", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping auth db: %w", err)
	}

	if err := ensureUser(ctx, db, adminEmail, adminPassword, "admin"); err != nil {
		return err
	}
	if err := ensureUser(ctx, db, userEmail, userPassword, "user"); err != nil {
		return err
	}

	log.Println("auth seed completed")
	return nil
}

func ensureUser(ctx context.Context, db *pgxpool.Pool, email, password, role string) error {
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return fmt.Errorf("check user exists (%s): %w", email, err)
	}
	if exists {
		log.Printf("seed: user already exists, skipping: %s", email)
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password for %s: %w", email, err)
	}

	_, err = db.Exec(ctx, `
        INSERT INTO users (id, email, password_hash, role, is_verified, is_active, is_deleted, created_at, updated_at)
        VALUES ($1, $2, $3, $4, TRUE, TRUE, FALSE, NOW(), NOW())
    `, uuid.New(), strings.ToLower(email), string(hash), role)
	if err != nil {
		return fmt.Errorf("insert user %s: %w", email, err)
	}

	log.Printf("seed: created %s user: %s", role, email)
	return nil
}
