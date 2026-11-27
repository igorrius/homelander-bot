package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

var ctx = context.Background()

func ptr[T any](v T) *T { return &v }

// clock is a settable test clock.
type clock struct{ t time.Time }

func (c *clock) Now() time.Time          { return c.t }
func (c *clock) Advance(d time.Duration) { c.t = c.t.Add(d) }

type env struct {
	t       *testing.T
	db      sqlite.DB
	r       domain.Repos
	clock   *clock
	users   *domain.UserService
	catalog *domain.CatalogService
	data    *domain.DataService
	pages   *domain.PageService
	owner   domain.ResidencyStatus
	house   domain.House
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db := sqlitetest.New(t)
	r := sqlite.NewRepos(db)
	c := &clock{t: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)}
	e := &env{t: t, db: db, r: r, clock: c}
	e.users = &domain.UserService{R: r, Now: c.Now, Limits: ratelimit.New(c.Now), Region: "UA", PendingCap: 200}
	e.catalog = &domain.CatalogService{R: r, Now: c.Now, Locales: []string{"uk", "en"}}
	e.data = &domain.DataService{R: r, Now: c.Now, Limits: ratelimit.New(c.Now), Locales: []string{"uk", "en"}}
	e.pages = &domain.PageService{R: r, Now: c.Now, Secret: []byte("test-secret")}
	sts, _ := r.Statuses.List(ctx, false)
	e.owner = sts[0]
	e.house = e.mkHouse("1", 1, 10)
	return e
}

func (e *env) mkHouse(name string, from, to int) domain.House {
	e.t.Helper()
	h := domain.House{Name: name, ApartmentFrom: from, ApartmentTo: to}
	if err := e.r.Houses.Create(ctx, &h); err != nil {
		e.t.Fatal(err)
	}
	return h
}

func (e *env) application(phone string) domain.Application {
	return domain.Application{Phone: phone, FirstName: "Тест", LastName: "Мешканець", HouseID: e.house.ID,
		Apartment: 3, StatusID: e.owner.ID, Consent: true, Locale: "uk"}
}

// mkUser stores a user directly.
func (e *env) mkUser(phone string, mod ...func(*domain.User)) domain.User {
	e.t.Helper()
	u := domain.User{Phone: phone, FirstName: "F", LastName: "L", Status: domain.StatusActive, Source: domain.SourceAdmin, IsResident: true}
	for _, m := range mod {
		m(&u)
	}
	if err := e.r.Users.Create(ctx, &u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *env) link(u domain.User, external string) domain.Channel {
	e.t.Helper()
	c := domain.Channel{UserID: u.ID, Type: "telegram", ExternalID: external, ChatID: external}
	if err := e.r.Channels.Create(ctx, &c); err != nil {
		e.t.Fatal(err)
	}
	return c
}

func (e *env) session(u domain.User) {
	e.t.Helper()
	s := domain.Session{UserID: u.ID, TokenHash: []byte(u.Phone), CreatedAt: e.clock.t, LastSeenAt: e.clock.t, ExpiresAt: e.clock.t.Add(time.Hour)}
	if err := e.r.Sessions.Create(ctx, &s); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) sessions(u domain.User) int {
	var n int
	_ = e.db.QueryRow(`SELECT count(*) FROM sessions WHERE user_id = ?`, u.ID).Scan(&n)
	return n
}

func (e *env) get(id int64) domain.User {
	e.t.Helper()
	u, err := e.r.Users.Get(ctx, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *env) auditCount(action string) int {
	var n int
	_ = e.db.QueryRow(`SELECT count(*) FROM audit_log WHERE action = ?`, action).Scan(&n)
	return n
}

func admin(u *domain.User) { u.IsAdmin = true }

var public = domain.Actor{IP: "10.0.0.1"}

func actor(u domain.User) domain.Actor { return domain.Actor{UserID: &u.ID, IP: "10.0.0.2"} }
