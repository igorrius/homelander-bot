package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

var ctx = context.Background()

func ptr[T any](v T) *T { return &v }

func mkUser(t *testing.T, users sqlite.Users, phone, first, last string, mod ...func(*domain.User)) domain.User {
	t.Helper()
	u := domain.User{Phone: phone, FirstName: first, LastName: last, Status: domain.StatusActive, Source: domain.SourceAdmin, IsResident: true}
	for _, m := range mod {
		m(&u)
	}
	if err := users.Create(ctx, &u); err != nil {
		t.Fatalf("Create %s: %v", phone, err)
	}
	return u
}

func TestUserCreateGet(t *testing.T) {
	users := sqlite.Users{DB: sqlitetest.New(t)}
	approved := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	u := mkUser(t, users, "+380500000001", "Іван", "Петренко", func(u *domain.User) {
		u.IsAdmin = true
		u.Locale = "uk"
		u.ApprovedAt = &approved
	})
	if u.ID == 0 || u.CreatedAt.IsZero() || u.UpdatedAt.IsZero() {
		t.Fatalf("Create did not set ID/timestamps: %+v", u)
	}
	got, err := users.Get(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phone != u.Phone || got.FirstName != "Іван" || !got.IsAdmin || !got.IsResident ||
		got.Status != domain.StatusActive || got.Locale != "uk" || got.ApprovedAt == nil || !got.ApprovedAt.Equal(approved) {
		t.Errorf("Get = %+v", got)
	}
	byPhone, err := users.GetByPhone(ctx, "+380500000001")
	if err != nil || byPhone.ID != u.ID {
		t.Errorf("GetByPhone = %+v, %v", byPhone, err)
	}
	got.LastName = "Петренко-Нові"
	got.Status = domain.StatusDisabled
	if err := users.Update(ctx, &got); err != nil {
		t.Fatal(err)
	}
	again, _ := users.Get(ctx, u.ID)
	if again.LastName != "Петренко-Нові" || again.Status != domain.StatusDisabled {
		t.Errorf("after Update = %+v", again)
	}
}

func TestUserPhoneUnique(t *testing.T) {
	users := sqlite.Users{DB: sqlitetest.New(t)}
	mkUser(t, users, "+380500000001", "A", "B")
	u := domain.User{Phone: "+380500000001", Status: domain.StatusActive, Source: domain.SourceAdmin}
	if err := users.Create(ctx, &u); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate phone err = %v, want ErrConflict", err)
	}
}

func TestUserGetMissing(t *testing.T) {
	users := sqlite.Users{DB: sqlitetest.New(t)}
	if _, err := users.Get(ctx, 42); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get err = %v", err)
	}
	if _, err := users.GetByPhone(ctx, "+380500000099"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetByPhone err = %v", err)
	}
}

func names(us []domain.User) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.FirstName + " " + u.LastName
	}
	return out
}

func TestUserListSearch(t *testing.T) {
	users := sqlite.Users{DB: sqlitetest.New(t)}
	mkUser(t, users, "+380500000001", "Іван", "Петренко")
	mkUser(t, users, "+380500000002", "Olena", "Shevchenko")
	mkUser(t, users, "+380500000003", "Петро", "Іваненко")
	cases := map[string]int{"петренко": 1, "іван": 2, "0500000002": 1, "Петренко Іван": 1, "Іван Петренко": 1, "OLENA": 1, "": 3, "nobody": 0}
	for q, want := range cases {
		got, total, err := users.List(ctx, domain.UserFilter{Query: q})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want || total != want {
			t.Errorf("query %q: %v (total %d), want %d", q, names(got), total, want)
		}
	}
	page, total, _ := users.List(ctx, domain.UserFilter{Limit: 2})
	if len(page) != 2 || total != 3 {
		t.Errorf("paged: %d items, total %d", len(page), total)
	}
	// Ordered by last name.
	all, _, _ := users.List(ctx, domain.UserFilter{})
	if all[0].LastName != "Shevchenko" || all[1].LastName != "Іваненко" {
		t.Errorf("order = %v", names(all))
	}
}

func TestUserListFilters(t *testing.T) {
	users := sqlite.Users{DB: sqlitetest.New(t)}
	mkUser(t, users, "+380500000001", "A", "Admin", func(u *domain.User) { u.IsAdmin = true; u.IsResident = false })
	mkUser(t, users, "+380500000002", "B", "Pending", func(u *domain.User) { u.Status = domain.StatusPendingApproval })
	mkUser(t, users, "+380500000003", "C", "Resident")
	check := func(name string, f domain.UserFilter, want int) {
		t.Helper()
		got, total, err := users.List(ctx, f)
		if err != nil || len(got) != want || total != want {
			t.Errorf("%s: %v total %d err %v, want %d", name, names(got), total, err, want)
		}
	}
	check("admins", domain.UserFilter{IsAdmin: ptr(true)}, 1)
	check("residents", domain.UserFilter{IsResident: ptr(true)}, 2)
	check("pending", domain.UserFilter{Status: ptr(domain.StatusPendingApproval)}, 1)
}

func TestCountActiveAdmins(t *testing.T) {
	users := sqlite.Users{DB: sqlitetest.New(t)}
	mkUser(t, users, "+380500000001", "A", "A", func(u *domain.User) { u.IsAdmin = true })
	mkUser(t, users, "+380500000002", "B", "B", func(u *domain.User) { u.IsAdmin = true; u.Status = domain.StatusDisabled })
	mkUser(t, users, "+380500000003", "C", "C", func(u *domain.User) { u.Status = domain.StatusPendingApproval })
	if n, err := users.CountActiveAdmins(ctx); err != nil || n != 1 {
		t.Errorf("CountActiveAdmins = %d, %v", n, err)
	}
	if n, err := users.CountByStatus(ctx, domain.StatusPendingApproval); err != nil || n != 1 {
		t.Errorf("CountByStatus = %d, %v", n, err)
	}
}

func TestChannelUniqueness(t *testing.T) {
	db := sqlitetest.New(t)
	users, chans := sqlite.Users{DB: db}, sqlite.Channels{DB: db}
	a := mkUser(t, users, "+380500000001", "A", "A")
	b := mkUser(t, users, "+380500000002", "B", "B")
	c := domain.Channel{UserID: a.ID, Type: "telegram", ExternalID: "111", ChatID: "111", Username: "a"}
	if err := chans.Create(ctx, &c); err != nil || c.ID == 0 || c.LinkedAt.IsZero() {
		t.Fatalf("Create = %+v, %v", c, err)
	}
	dupExt := domain.Channel{UserID: b.ID, Type: "telegram", ExternalID: "111", ChatID: "111"}
	if err := chans.Create(ctx, &dupExt); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("same external id err = %v", err)
	}
	second := domain.Channel{UserID: a.ID, Type: "telegram", ExternalID: "222", ChatID: "222"}
	if err := chans.Create(ctx, &second); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("second telegram channel err = %v", err)
	}
	got, err := chans.GetByExternal(ctx, "telegram", "111")
	if err != nil || got.UserID != a.ID {
		t.Errorf("GetByExternal = %+v, %v", got, err)
	}
	list, _ := chans.ListByUser(ctx, a.ID)
	if len(list) != 1 {
		t.Errorf("ListByUser = %v", list)
	}
	if err := chans.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := chans.GetByExternal(ctx, "telegram", "111"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("after delete err = %v", err)
	}
}

func TestSeededStatuses(t *testing.T) {
	db := sqlitetest.New(t)
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM residency_statuses WHERE code IN ('owner','tenant')`).Scan(&n); err != nil || n != 2 {
		t.Errorf("seeded statuses = %d, %v", n, err)
	}
}
