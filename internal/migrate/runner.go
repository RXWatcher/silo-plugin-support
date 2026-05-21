// Package migrate runs the plugin's schema migrations on start.
// Files are embedded so the binary is self-contained.
package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed files/*.sql
var migrationsFS embed.FS

// Run applies every pending migration against the database at dsn.
// Returns nil on success (including "no change").
func Run(ctx context.Context, dsn string) error {
	src, err := iofs.New(migrationsFS, "files")
	if err != nil {
		return fmt.Errorf("load migrations FS: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("open migrate: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	_ = ctx // reserved for future cancellation
	return nil
}
