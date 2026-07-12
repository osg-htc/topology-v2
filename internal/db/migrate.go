package db

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	// pgx stdlib shim, used only so goose can speak database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations applies all pending goose migrations from the embedded FS.
// It is called at server startup so a fresh database is always schema-current.
func RunMigrations(databaseURL string) error {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("opening sql db for migrations: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

// MigrationStatus prints the goose migration status.
func MigrationStatus(databaseURL string) error {
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("opening sql db for migrations: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}
	return goose.Status(sqlDB, "migrations")
}
