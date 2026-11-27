package sqlite_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

type catalog struct {
	users  sqlite.Users
	houses sqlite.Houses
	stats  sqlite.Statuses
	res    sqlite.Residencies
}

func newCatalog(t *testing.T) catalog {
	db := sqlitetest.New(t)
	return catalog{sqlite.Users{DB: db}, sqlite.Houses{DB: db}, sqlite.Statuses{DB: db}, sqlite.Residencies{DB: db}}
}

func (c catalog) status(t *testing.T, code string) domain.ResidencyStatus {
	t.Helper()
	all, err := c.stats.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		if s.Code == code {
			return s
		}
	}
	t.Fatalf("no status %s", code)
	return domain.ResidencyStatus{}
}

func (c catalog) house(t *testing.T, name string, from, to int) domain.House {
	t.Helper()
	h := domain.House{Name: name, ApartmentFrom: from, ApartmentTo: to}
	if err := c.houses.Create(ctx, &h); err != nil {
		t.Fatal(err)
	}
	return h
}

func (c catalog) live(t *testing.T, u domain.User, h domain.House, apt int, st domain.ResidencyStatus) domain.Residency {
	t.Helper()
	r := domain.Residency{UserID: u.ID, HouseID: h.ID, Apartment: apt, StatusID: st.ID}
	if err := c.res.Create(ctx, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestHouseCRUD(t *testing.T) {
	c := newCatalog(t)
	lat, lon, floors := 50.45, 30.52, 9
	h := domain.House{Name: "1", Address: "Test street 1", Latitude: &lat, Longitude: &lon, Floors: &floors, ApartmentFrom: 1, ApartmentTo: 120, SortOrder: 2}
	if err := c.houses.Create(ctx, &h); err != nil || h.ID == 0 {
		t.Fatalf("Create = %+v, %v", h, err)
	}
	got, err := c.houses.Get(ctx, h.ID)
	if err != nil || !reflect.DeepEqual(got, h) {
		t.Fatalf("Get = %+v, %v; want %+v", got, err, h)
	}
	got.Name, got.Latitude, got.ApartmentTo = "1A", nil, 130
	if err := c.houses.Update(ctx, &got); err != nil {
		t.Fatal(err)
	}
	again, _ := c.houses.Get(ctx, h.ID)
	if again.Name != "1A" || again.Latitude != nil || again.ApartmentTo != 130 {
		t.Errorf("after Update = %+v", again)
	}
	if _, err := c.houses.Get(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("missing err = %v", err)
	}
}

func TestHouseListHidesArchived(t *testing.T) {
	c := newCatalog(t)
	c.house(t, "B", 1, 10)
	a := c.house(t, "A", 1, 10)
	a.Archived = true
	_ = c.houses.Update(ctx, &a)
	vis, _ := c.houses.List(ctx, false)
	all, _ := c.houses.List(ctx, true)
	if len(vis) != 1 || vis[0].Name != "B" || len(all) != 2 || all[0].Name != "A" {
		t.Errorf("visible %v, all %v", vis, all)
	}
}

func TestStatusNamesRoundTrip(t *testing.T) {
	c := newCatalog(t)
	s := domain.ResidencyStatus{Code: "co_owner", Names: domain.LocalizedText{"uk": "Співвласник", "en": "Co-owner"}, SortOrder: 3}
	if err := c.stats.Create(ctx, &s); err != nil {
		t.Fatal(err)
	}
	got, err := c.stats.Get(ctx, s.ID)
	if err != nil || !reflect.DeepEqual(got, s) {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	dup := domain.ResidencyStatus{Code: "co_owner", Names: domain.LocalizedText{"uk": "x"}}
	if err := c.stats.Create(ctx, &dup); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate code err = %v", err)
	}
	got.Archived = true
	_ = c.stats.Update(ctx, &got)
	vis, _ := c.stats.List(ctx, false)
	if len(vis) != 2 {
		t.Errorf("visible statuses = %d", len(vis))
	}
}

func TestResidencyDuplicate(t *testing.T) {
	c := newCatalog(t)
	h := c.house(t, "1", 1, 10)
	owner := c.status(t, "owner")
	a := mkUser(t, c.users, "+380500000001", "A", "A")
	b := mkUser(t, c.users, "+380500000002", "B", "B")
	r := c.live(t, a, h, 3, owner)
	dup := domain.Residency{UserID: a.ID, HouseID: h.ID, Apartment: 3, StatusID: owner.ID}
	if err := c.res.Create(ctx, &dup); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("duplicate err = %v", err)
	}
	c.live(t, b, h, 3, owner) // a second person in the same apartment is fine
	r.Apartment = 4
	if err := c.res.Update(ctx, &r); err != nil {
		t.Fatal(err)
	}
	list, _ := c.res.ListByUser(ctx, a.ID)
	if len(list) != 1 || list[0].Apartment != 4 {
		t.Errorf("ListByUser = %+v", list)
	}
	if err := c.res.Delete(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.res.Get(ctx, r.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("after delete err = %v", err)
	}
}

func TestHouseStats(t *testing.T) {
	c := newCatalog(t)
	h := c.house(t, "1", 1, 10)
	other := c.house(t, "2", 11, 20)
	owner, tenant := c.status(t, "owner"), c.status(t, "tenant")
	set := func(s domain.UserStatus) func(*domain.User) { return func(u *domain.User) { u.Status = s } }
	c.live(t, mkUser(t, c.users, "+380500000001", "A", "A"), h, 3, owner)
	c.live(t, mkUser(t, c.users, "+380500000002", "B", "B"), h, 3, tenant)
	c.live(t, mkUser(t, c.users, "+380500000003", "C", "C", set(domain.StatusPendingApproval)), h, 5, owner)
	c.live(t, mkUser(t, c.users, "+380500000004", "D", "D", set(domain.StatusRejected)), h, 7, owner)
	c.live(t, mkUser(t, c.users, "+380500000005", "E", "E", set(domain.StatusDisabled)), h, 7, owner)
	c.live(t, mkUser(t, c.users, "+380500000006", "F", "F", func(u *domain.User) { u.IsResident = false; u.IsAdmin = true }), h, 8, owner)
	c.live(t, mkUser(t, c.users, "+380500000007", "G", "G"), other, 12, owner)
	got, err := c.houses.Stats(ctx, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.HouseStats{
		ApartmentsTotal: 10, ApartmentsWithResidents: 2, People: 3,
		ByResidencyStatus: map[int64]int{owner.ID: 2, tenant.ID: 1},
		ByUserStatus:      map[domain.UserStatus]int{domain.StatusActive: 2, domain.StatusPendingApproval: 1},
		Apartments:        map[int]int{3: 2, 5: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Stats = %+v\nwant   %+v", got, want)
	}
	// HouseID filter on the user list (Task 3's filter, needs houses).
	us, total, _ := c.users.List(ctx, domain.UserFilter{HouseID: &other.ID})
	if total != 1 || us[0].FirstName != "G" {
		t.Errorf("house filter = %v", names(us))
	}
}

func TestResidentsOfApartment(t *testing.T) {
	c := newCatalog(t)
	h := c.house(t, "1", 1, 10)
	owner := c.status(t, "owner")
	c.live(t, mkUser(t, c.users, "+380500000001", "A", "Bee"), h, 3, owner)
	c.live(t, mkUser(t, c.users, "+380500000002", "B", "Ant"), h, 3, owner)
	c.live(t, mkUser(t, c.users, "+380500000003", "C", "Cat"), h, 4, owner)
	got, err := c.houses.Residents(ctx, h.ID, 3)
	if err != nil || len(got) != 2 || got[0].User.LastName != "Ant" || got[0].Residency.Apartment != 3 {
		t.Errorf("Residents = %+v, %v", got, err)
	}
}

func TestResidenciesOutside(t *testing.T) {
	c := newCatalog(t)
	h := c.house(t, "1", 1, 10)
	owner := c.status(t, "owner")
	c.live(t, mkUser(t, c.users, "+380500000001", "A", "A"), h, 3, owner)
	c.live(t, mkUser(t, c.users, "+380500000002", "B", "B"), h, 8, owner)
	if n, err := c.houses.ResidenciesOutside(ctx, h.ID, 1, 4); err != nil || n != 1 {
		t.Errorf("outside 1-4 = %d, %v", n, err)
	}
	if n, _ := c.houses.ResidenciesOutside(ctx, h.ID, 1, 10); n != 0 {
		t.Errorf("outside 1-10 = %d", n)
	}
}
