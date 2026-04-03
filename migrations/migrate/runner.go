package migrate

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Runner struct {
	SourceDir string
	DBURL     string
}

func (r Runner) Up() error {
	m, err := migrate.New(r.SourceDir, r.DBURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer closeMigrate(m)

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply up migrations: %w", err)
	}

	log.Println("migrations applied")
	return nil
}

func (r Runner) Down() error {
	m, err := migrate.New(r.SourceDir, r.DBURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer closeMigrate(m)

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply down migrations: %w", err)
	}

	log.Println("migrations reverted")
	return nil
}

func (r Runner) Force(version int) error {
	m, err := migrate.New(r.SourceDir, r.DBURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer closeMigrate(m)

	if err := m.Force(version); err != nil {
		return fmt.Errorf("force migration version: %w", err)
	}

	log.Printf("forced migration version to %d", version)
	return nil
}

func closeMigrate(m *migrate.Migrate) {
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		log.Printf("warning: close migration source: %v", srcErr)
	}
	if dbErr != nil {
		log.Printf("warning: close migration db: %v", dbErr)
	}
}
