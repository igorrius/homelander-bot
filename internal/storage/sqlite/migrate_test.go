package sqlite

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
)

func file(body string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(body)} }

func TestMigrateAppliesInOrderOnce(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)
	fsys := fstest.MapFS{
		"0002_b.sql": file(`INSERT INTO log(name) VALUES ('b');`),
		"0001_a.sql": file(`CREATE TABLE log(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO log(name) VALUES ('a');`),
	}
	for range 2 {
		if err := Migrate(ctx, db, fsys); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
	}
	rows, err := db.Query(`SELECT name FROM log ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if strings.Join(got, ",") != "a,b" {
		t.Errorf("log = %v, want [a b]", got)
	}
}

func TestMigrateFailedFileRollsBack(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)
	fsys := fstest.MapFS{
		"0001_a.sql":      file(`CREATE TABLE a(id INTEGER);`),
		"0002_broken.sql": file(`CREATE TABLE x(id INTEGER); SELECT * FROM missing;`),
	}
	err := Migrate(ctx, db, fsys)
	if err == nil || !strings.Contains(err.Error(), "0002") {
		t.Fatalf("Migrate error = %v, want one naming 0002", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'x'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("table x exists after failed migration (n=%d, err=%v)", n, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 1 {
		t.Errorf("schema_migrations rows = %d (%v), want 1", n, err)
	}
	fsys["0002_broken.sql"] = file(`CREATE TABLE x(id INTEGER);`)
	for range 2 {
		if err := Migrate(ctx, db, fsys); err != nil {
			t.Fatalf("Migrate after fix: %v", err)
		}
	}
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 2 {
		t.Errorf("schema_migrations rows = %d (%v), want 2", n, err)
	}
}

func TestMigrateRejectsBadNames(t *testing.T) {
	db := openTemp(t)
	err := Migrate(context.Background(), db, fstest.MapFS{"1_a.sql": file(`SELECT 1;`)})
	if err == nil || !strings.Contains(err.Error(), "1_a.sql") {
		t.Fatalf("Migrate error = %v, want one naming 1_a.sql", err)
	}
}

func TestMigrateRejectsDuplicateVersions(t *testing.T) {
	db := openTemp(t)
	err := Migrate(context.Background(), db, fstest.MapFS{
		"0001_a.sql": file(`SELECT 1;`),
		"0001_b.sql": file(`SELECT 1;`),
	})
	if err == nil || !strings.Contains(err.Error(), "0001") {
		t.Fatalf("Migrate error = %v, want duplicate version error", err)
	}
}

func TestEmbeddedMigrationsApply(t *testing.T) {
	db := openTemp(t)
	if err := Migrate(context.Background(), db, Migrations); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'settings'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("settings table missing (n=%d, err=%v)", n, err)
	}
}
