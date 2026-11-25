package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"cloudbin-migrations/migrate"
	"cloudbin-migrations/seed"
	"cloudbin-shared/env"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	migrateAction := flag.String("migrate", "", "migration action: up|down")
	seedFlag := flag.Bool("seed", false, "run seed logic")
	target := flag.String("target", "auth", "target db: auth|object")
	flag.Parse()

	env.LoadFile(".env")
	env.LoadFile("../.env")

	if *migrateAction == "" && !*seedFlag {
		log.Fatalf("no action provided. use -migrate=up|down and/or -seed")
	}

	if *migrateAction != "" {
		if err := runMigrate(*target, *migrateAction); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
	}

	if *seedFlag {
		if err := runSeed(*target); err != nil {
			log.Fatalf("seed failed: %v", err)
		}
	}
}

func runMigrate(target, action string) error {
	if action != "up" && action != "down" {
		return fmt.Errorf("unsupported migrate action %q (use up or down)", action)
	}

	dsnEnv, dir, err := targetConfig(target)
	if err != nil {
		return err
	}

	dsn := strings.TrimSpace(os.Getenv(dsnEnv))
	if dsn == "" {
		return fmt.Errorf("%s is not set", dsnEnv)
	}

	if err := prepareLegacySchemaMigrations(dsn); err != nil {
		return err
	}

	r := migrate.Runner{
		SourceDir: "file://" + dir,
		DBURL:     dsn,
	}

	if action == "up" {
		return r.Up()
	}
	return r.Down()
}

func prepareLegacySchemaMigrations(dsn string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect db for migration metadata: %w", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping db for migration metadata: %w", err)
	}

	var hasAppliedAt bool
	err = db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'schema_migrations'
			  AND column_name = 'applied_at'
		)
	`).Scan(&hasAppliedAt)
	if err != nil {
		return fmt.Errorf("check schema_migrations shape: %w", err)
	}

	if !hasAppliedAt {
		return nil
	}

	log.Println("detected legacy schema_migrations table, renaming to schema_migrations_legacy")
	_, err = db.Exec(ctx, `ALTER TABLE IF EXISTS schema_migrations RENAME TO schema_migrations_legacy`)
	if err != nil {
		return fmt.Errorf("rename legacy schema_migrations table: %w", err)
	}

	return nil
}

func runSeed(target string) error {
	switch target {
	case "auth":
		return seed.RunAuth()
	case "object":
		return fmt.Errorf("object seed is not implemented yet")
	default:
		return fmt.Errorf("unknown target %q (use auth or object)", target)
	}
}

func targetConfig(target string) (dsnEnv string, dir string, err error) {
	switch target {
	case "auth":
		return "AUTH_DB_DSN", "./auth", nil
	case "object":
		return "OBJECT_DB_DSN", "./object", nil
	default:
		return "", "", fmt.Errorf("unknown target %q (use auth or object)", target)
	}
}
