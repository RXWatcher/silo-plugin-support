// Package migrate runs the plugin's schema migrations on start.
// Files are embedded so the binary is self-contained.
//
// Uses the pgx/v5 migrate driver (matching the pgxpool used by store)
// so the whole plugin reads its DB through a single driver and DSN
// options like pool_max_conns / connect_timeout are parsed the same
// way everywhere.
package migrate

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed files/*.sql
var migrationsFS embed.FS

// Run applies every pending migration against the database at dsn.
// Returns nil on success (including "no change"). ctx is accepted
// for API compatibility — golang-migrate does not yet support per-
// migration cancellation, so it's currently unused.
func Run(_ context.Context, dsn string) error {
	src, err := iofs.New(migrationsFS, "files")
	if err != nil {
		return fmt.Errorf("load migrations FS: %w", err)
	}

	// golang-migrate's pgx/v5 driver registers under the "pgx5" scheme.
	// Rewrite "postgres://" / "postgresql://" so the same DSN works
	// for both pgxpool (which accepts the standard prefixes) and the
	// migrator.
	driverDSN := dsn
	for _, p := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(driverDSN, p) {
			driverDSN = "pgx5://" + driverDSN[len(p):]
			break
		}
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, driverDSN)
	if err != nil {
		return fmt.Errorf("open migrate: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
