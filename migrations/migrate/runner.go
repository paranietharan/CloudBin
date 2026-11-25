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
	return r.run("up", (*migrate.Migrate).Up)
}

func (r Runner) Down() error {
	return r.run("down", (*migrate.Migrate).Down)
}

func (r Runner) run(direction string, apply func(*migrate.Migrate) error) error {
	m, err := migrate.New(r.SourceDir, r.DBURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer closeMigrate(m)

	if err := apply(m); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply %s migrations: %w", direction, err)
	}
	log.Printf("%s migrations completed", direction)
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
