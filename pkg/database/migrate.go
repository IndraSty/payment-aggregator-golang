package database

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// newMigrate creates a migrate instance pointed at ./migrations directory.
func newMigrate(databaseURL string) (*migrate.Migrate, error) {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	return m, nil
}
