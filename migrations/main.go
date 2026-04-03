package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloudbin-migrations/migrate"
	"cloudbin-migrations/seed"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	migrateAction := flag.String("migrate", "", "migration action: up|down")
	seedFlag := flag.Bool("seed", false, "run seed logic")
	target := flag.String("target", "auth", "target db: auth|object")
	flag.Parse()

	root, err := projectRoot()
	if err != nil {
		log.Fatalf("failed to read working directory: %v", err)
	}
	loadDotEnv(filepath.Join(root, "migrations", ".env"))
	loadDotEnv(filepath.Join(root, ".env"))

	if *migrateAction == "" && !*seedFlag {
		log.Fatalf("no action provided. use -migrate=up|down and/or -seed")
	}

	if *migrateAction != "" {
		if err := runMigrate(*target, *migrateAction, root); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
	}

	if *seedFlag {
		if err := runSeed(*target); err != nil {
			log.Fatalf("seed failed: %v", err)
		}
	}
}

func runMigrate(target, action, root string) error {
	if action != "up" && action != "down" {
		return fmt.Errorf("unsupported migrate action %q (use up or down)", action)
	}

	dsnEnv, dir, err := targetConfig(target, root)
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

	switch action {
	case "up":
		return r.Up()
	case "down":
		return r.Down()
	default:
		return fmt.Errorf("unsupported migrate action %q", action)
	}
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

func targetConfig(target, root string) (dsnEnv string, dir string, err error) {
	switch target {
	case "auth":
		return "AUTH_DB_DSN", filepath.Join(root, "migrations", "auth"), nil
	case "object":
		return "OBJECT_DB_DSN", filepath.Join(root, "migrations", "object"), nil
	default:
		return "", "", fmt.Errorf("unknown target %q (use auth or object)", target)
	}
}

func projectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if filepath.Base(cwd) == "migrations" {
		return filepath.Dir(cwd), nil
	}
	return cwd, nil
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("warning: failed to scan %s: %v", path, err)
	}
}
