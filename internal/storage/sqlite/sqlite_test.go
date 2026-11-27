package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpenCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "db.sqlite")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
}

func TestOpenSetsPragmas(t *testing.T) {
	db := openTemp(t)
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestInTxRollsBackOnError(t *testing.T) {
	ctx := context.Background()
	raw := openTemp(t)
	if err := Migrate(ctx, raw, Migrations); err != nil {
		t.Fatal(err)
	}
	db := DB{raw}
	insert := func(ctx context.Context, key string) error {
		_, err := db.Q(ctx).ExecContext(ctx,
			`INSERT INTO settings(key, value, updated_at) VALUES (?, '1', '2026-01-01T00:00:00Z')`, key)
		return err
	}
	boom := errors.New("boom")
	err := db.InTx(ctx, func(ctx context.Context) error {
		if err := insert(ctx, "rolled-back"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx error = %v, want boom", err)
	}
	if err := db.InTx(ctx, func(ctx context.Context) error {
		// A nested InTx joins the outer transaction.
		return db.InTx(ctx, func(ctx context.Context) error { return insert(ctx, "kept") })
	}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := raw.QueryRow(`SELECT count(*) FROM settings WHERE key = 'rolled-back'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("rolled-back row count = %d (%v), want 0", n, err)
	}
	if err := raw.QueryRow(`SELECT count(*) FROM settings WHERE key = 'kept'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("kept row count = %d (%v), want 1", n, err)
	}
}
