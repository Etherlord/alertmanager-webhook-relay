// Package testutil provides shared test helpers.
package testutil

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// ApplyMigrations runs goose migrations from migrationsDir against the given db.
// Uses goose.NewProvider to avoid mutating global state (safe for parallel tests).
func ApplyMigrations(t *testing.T, db *sql.DB, migrationsDir string) {
	t.Helper()

	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		db,
		os.DirFS(migrationsDir),
		goose.WithLogger(goose.NopLogger()),
	)
	require.NoError(t, err)

	_, err = provider.Up(context.Background())
	require.NoError(t, err)
}
