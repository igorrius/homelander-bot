// Package sqlitetest opens migrated throwaway databases for tests.
package sqlitetest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
)

// New opens a migrated database in t.TempDir() and closes it when the test ends.
func New(t testing.TB) sqlite.DB {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlite.Migrate(ctx, db, sqlite.Migrations); err != nil {
		t.Fatal(err)
	}
	return sqlite.DB{DB: db}
}
