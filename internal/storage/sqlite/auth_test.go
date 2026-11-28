package sqlite_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

var t0 = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

func TestCodeReplaceKeepsOne(t *testing.T) {
	db := sqlitetest.New(t)
	u := mkUser(t, sqlite.Users{DB: db}, "+380500000001", "A", "A")
	codes := sqlite.Codes{DB: db}
	first := domain.LoginCode{UserID: u.ID, CodeHash: []byte("one"), ExpiresAt: t0.Add(5 * time.Minute)}
	second := domain.LoginCode{UserID: u.ID, CodeHash: []byte("two"), ExpiresAt: t0.Add(5 * time.Minute), IP: "1.2.3.4"}
	if err := codes.Replace(ctx, &first); err != nil {
		t.Fatal(err)
	}
	if err := codes.Replace(ctx, &second); err != nil {
		t.Fatal(err)
	}
	got, err := codes.Active(ctx, u.ID, t0)
	if err != nil || string(got.CodeHash) != "two" || got.IP != "1.2.3.4" {
		t.Fatalf("Active = %+v, %v", got, err)
	}
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM login_codes`).Scan(&n)
	if n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestCodeActiveIgnoresExpiredAndUsed(t *testing.T) {
	db := sqlitetest.New(t)
	u := mkUser(t, sqlite.Users{DB: db}, "+380500000001", "A", "A")
	codes := sqlite.Codes{DB: db}
	c := domain.LoginCode{UserID: u.ID, CodeHash: []byte("x"), ExpiresAt: t0.Add(time.Minute)}
	_ = codes.Replace(ctx, &c)
	if _, err := codes.Active(ctx, u.ID, t0.Add(2*time.Minute)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expired err = %v", err)
	}
	if n, err := codes.IncAttempts(ctx, c.ID); err != nil || n != 1 {
		t.Errorf("IncAttempts = %d, %v", n, err)
	}
	if ok, _ := codes.MarkUsed(ctx, c.ID, t0); !ok {
		t.Fatal("MarkUsed false")
	}
	if _, err := codes.Active(ctx, u.ID, t0); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("used err = %v", err)
	}
}

func TestMarkUsedOnlyOnce(t *testing.T) {
	db := sqlitetest.New(t)
	u := mkUser(t, sqlite.Users{DB: db}, "+380500000001", "A", "A")
	codes := sqlite.Codes{DB: db}
	c := domain.LoginCode{UserID: u.ID, CodeHash: []byte("x"), ExpiresAt: t0.Add(time.Minute)}
	_ = codes.Replace(ctx, &c)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if ok, err := codes.MarkUsed(ctx, c.ID, t0); err == nil && ok {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Errorf("MarkUsed succeeded %d times", wins.Load())
	}
}

func TestSessionLifecycle(t *testing.T) {
	db := sqlitetest.New(t)
	u := mkUser(t, sqlite.Users{DB: db}, "+380500000001", "A", "A")
	ss := sqlite.Sessions{DB: db}
	s := domain.Session{UserID: u.ID, TokenHash: []byte("h1"), CreatedAt: t0, LastSeenAt: t0, ExpiresAt: t0.Add(time.Hour), IP: "1.1.1.1", UserAgent: "test"}
	if err := ss.Create(ctx, &s); err != nil || s.ID == 0 {
		t.Fatalf("Create = %+v, %v", s, err)
	}
	got, err := ss.GetByHash(ctx, []byte("h1"))
	if err != nil || got.UserID != u.ID || !got.ExpiresAt.Equal(s.ExpiresAt) || got.UserAgent != "test" {
		t.Fatalf("GetByHash = %+v, %v", got, err)
	}
	if err := ss.Touch(ctx, s.ID, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ = ss.GetByHash(ctx, []byte("h1"))
	if !got.LastSeenAt.Equal(t0.Add(time.Minute)) {
		t.Errorf("LastSeenAt = %v", got.LastSeenAt)
	}
	_ = ss.Delete(ctx, s.ID)
	if _, err := ss.GetByHash(ctx, []byte("h1")); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("after delete err = %v", err)
	}
}

func TestDeleteSessionsByUserAndExpired(t *testing.T) {
	db := sqlitetest.New(t)
	users := sqlite.Users{DB: db}
	a := mkUser(t, users, "+380500000001", "A", "A")
	b := mkUser(t, users, "+380500000002", "B", "B")
	ss := sqlite.Sessions{DB: db}
	for i, s := range []domain.Session{
		{UserID: a.ID, TokenHash: []byte("a1"), ExpiresAt: t0.Add(time.Hour)},
		{UserID: a.ID, TokenHash: []byte("a2"), ExpiresAt: t0.Add(time.Hour)},
		{UserID: b.ID, TokenHash: []byte("b1"), ExpiresAt: t0.Add(-time.Hour)},
		{UserID: b.ID, TokenHash: []byte("b2"), ExpiresAt: t0.Add(time.Hour)},
	} {
		s.CreatedAt, s.LastSeenAt = t0, t0
		if err := ss.Create(ctx, &s); err != nil {
			t.Fatal(i, err)
		}
	}
	if err := ss.DeleteByUser(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := ss.DeleteExpired(ctx, t0); err != nil || n != 1 {
		t.Errorf("DeleteExpired = %d, %v", n, err)
	}
	var left int
	_ = db.QueryRow(`SELECT count(*) FROM sessions`).Scan(&left)
	if left != 1 {
		t.Errorf("sessions left = %d, want 1", left)
	}
}

func TestLoginLinkLifecycle(t *testing.T) {
	db := sqlitetest.New(t)
	u := mkUser(t, sqlite.Users{DB: db}, "+380500000001", "A", "A")
	links := sqlite.LoginLinks{DB: db}
	l := domain.LoginLink{UserID: u.ID, TokenHash: []byte("h"), Next: "/my/feed/3", ExpiresAt: t0.Add(5 * time.Minute), CreatedAt: t0}
	if err := links.Create(ctx, &l); err != nil || l.ID == 0 {
		t.Fatalf("Create = %+v, %v", l, err)
	}
	got, err := links.GetByHash(ctx, []byte("h"))
	if err != nil || got.UserID != u.ID || got.Next != "/my/feed/3" || !got.ExpiresAt.Equal(l.ExpiresAt) || got.UsedAt != nil {
		t.Fatalf("GetByHash = %+v, %v", got, err)
	}
	if ok, err := links.MarkUsed(ctx, l.ID, t0); !ok || err != nil {
		t.Fatalf("first MarkUsed = %v, %v", ok, err)
	}
	if ok, _ := links.MarkUsed(ctx, l.ID, t0); ok {
		t.Error("second MarkUsed succeeded")
	}
	if got, _ := links.GetByHash(ctx, []byte("h")); got.UsedAt == nil {
		t.Error("UsedAt not stored")
	}
	if n, err := links.DeleteExpiredBefore(ctx, t0.Add(time.Minute)); n != 0 || err != nil {
		t.Errorf("deleted a live link: %d, %v", n, err)
	}
	if n, err := links.DeleteExpiredBefore(ctx, t0.Add(6*time.Minute)); n != 1 || err != nil {
		t.Errorf("DeleteExpiredBefore = %d, %v", n, err)
	}
	if _, err := links.GetByHash(ctx, []byte("h")); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("after delete err = %v", err)
	}
}
