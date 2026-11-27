package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func wantFields(t *testing.T, err error, want map[string]string) {
	t.Helper()
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError %v", err, want)
	}
	for f, r := range want {
		if ve.Fields[f] != r {
			t.Errorf("field %s = %q, want %q (all: %v)", f, ve.Fields[f], r, ve.Fields)
		}
	}
}

func TestApplyCreatesPendingResident(t *testing.T) {
	e := newEnv(t)
	if err := e.users.Apply(ctx, e.application("050 000 00 01"), public); err != nil {
		t.Fatal(err)
	}
	u, err := e.r.Users.GetByPhone(ctx, "+380500000001")
	if err != nil {
		t.Fatal(err)
	}
	if u.Status != domain.StatusPendingApproval || !u.IsResident || u.IsAdmin || u.Source != domain.SourceApplication || u.Locale != "uk" {
		t.Errorf("user = %+v", u)
	}
	res, _ := e.r.Residencies.ListByUser(ctx, u.ID)
	if len(res) != 1 || res[0].Apartment != 3 || res[0].HouseID != e.house.ID {
		t.Errorf("residencies = %+v", res)
	}
	if e.auditCount(domain.ActUserApplied) != 1 {
		t.Error("no audit entry")
	}
}

func TestApplyValidation(t *testing.T) {
	e := newEnv(t)
	archived := e.mkHouse("old", 1, 5)
	archived.Archived = true
	_ = e.r.Houses.Update(ctx, &archived)
	cases := []struct {
		mod  func(*domain.Application)
		want map[string]string
	}{
		{func(a *domain.Application) { a.Phone = "12" }, map[string]string{"phone": "invalid"}},
		{func(a *domain.Application) { a.FirstName = " " }, map[string]string{"firstName": "required"}},
		{func(a *domain.Application) { a.LastName = strings.Repeat("я", 101) }, map[string]string{"lastName": "too_long"}},
		{func(a *domain.Application) { a.HouseID = 999 }, map[string]string{"houseId": "invalid"}},
		{func(a *domain.Application) { a.HouseID = archived.ID }, map[string]string{"houseId": "invalid"}},
		{func(a *domain.Application) { a.Apartment = 11 }, map[string]string{"apartment": "out_of_range"}},
		{func(a *domain.Application) { a.StatusID = 999 }, map[string]string{"statusId": "invalid"}},
		{func(a *domain.Application) { a.Consent = false }, map[string]string{"consent": "required"}},
	}
	for i, c := range cases {
		a := e.application("+380500000001")
		c.mod(&a)
		// Invalid submissions count against the rate limit too, so each case gets its own IP.
		wantFields(t, e.users.Apply(ctx, a, domain.Actor{IP: "10.1.0." + string(rune('0'+i))}), c.want)
	}
}

func TestApplyKnownPhoneChangesNothing(t *testing.T) {
	e := newEnv(t)
	u := e.mkUser("+380500000001")
	if err := e.users.Apply(ctx, e.application("+380500000001"), public); err != nil {
		t.Fatalf("Apply = %v, want nil", err)
	}
	got := e.get(u.ID)
	res, _ := e.r.Residencies.ListByUser(ctx, u.ID)
	if got.FirstName != "F" || got.Status != domain.StatusActive || len(res) != 0 || e.auditCount(domain.ActUserApplied) != 0 {
		t.Errorf("known phone changed: %+v, residencies %v", got, res)
	}
}

func TestApplyRejectedReapplies(t *testing.T) {
	e := newEnv(t)
	u := e.mkUser("+380500000001", func(u *domain.User) { u.Status = domain.StatusRejected; u.IsResident = false })
	if err := e.users.Apply(ctx, e.application("+380500000001"), public); err != nil {
		t.Fatal(err)
	}
	got := e.get(u.ID)
	if got.Status != domain.StatusPendingApproval || got.FirstName != "Тест" || !got.IsResident {
		t.Errorf("reapplied = %+v", got)
	}
}

func TestApplyRateLimited(t *testing.T) {
	e := newEnv(t)
	for i := 1; i <= 5; i++ {
		if err := e.users.Apply(ctx, e.application("+38050000000"+string(rune('0'+i))), public); err != nil {
			t.Fatal(i, err)
		}
	}
	if err := e.users.Apply(ctx, e.application("+380500000009"), public); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("6th err = %v", err)
	}
}

func TestApplyPendingCap(t *testing.T) {
	e := newEnv(t)
	e.users.PendingCap = 2
	_ = e.users.Apply(ctx, e.application("+380500000001"), domain.Actor{IP: "a"})
	_ = e.users.Apply(ctx, e.application("+380500000002"), domain.Actor{IP: "b"})
	if err := e.users.Apply(ctx, e.application("+380500000003"), domain.Actor{IP: "c"}); !errors.Is(err, domain.ErrTryLater) {
		t.Errorf("over cap err = %v", err)
	}
}

func TestApplySecondResidentSameApartment(t *testing.T) {
	e := newEnv(t)
	if err := e.users.Apply(ctx, e.application("+380500000001"), public); err != nil {
		t.Fatal(err)
	}
	if err := e.users.Apply(ctx, e.application("+380500000002"), public); err != nil {
		t.Fatal(err)
	}
	st, _ := e.r.Houses.Stats(ctx, e.house.ID)
	if st.Apartments[3] != 2 {
		t.Errorf("apartment 3 has %d people", st.Apartments[3])
	}
}

func TestApproveReject(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	_ = e.users.Apply(ctx, e.application("+380500000001"), public)
	_ = e.users.Apply(ctx, e.application("+380500000002"), public)
	a, _ := e.r.Users.GetByPhone(ctx, "+380500000001")
	b, _ := e.r.Users.GetByPhone(ctx, "+380500000002")
	if err := e.users.Approve(ctx, a.ID, actor(adm)); err != nil {
		t.Fatal(err)
	}
	if err := e.users.Reject(ctx, b.ID, actor(adm)); err != nil {
		t.Fatal(err)
	}
	ga, gb := e.get(a.ID), e.get(b.ID)
	if ga.Status != domain.StatusAwaitingActivation || ga.ApprovedBy == nil || *ga.ApprovedBy != adm.ID || ga.ApprovedAt == nil {
		t.Errorf("approved = %+v", ga)
	}
	if gb.Status != domain.StatusRejected {
		t.Errorf("rejected = %+v", gb)
	}
	if err := e.users.Approve(ctx, a.ID, actor(adm)); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("approve twice err = %v", err)
	}
}

func TestAddUserAwaitsActivation(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	u, err := e.users.AddUser(ctx, domain.NewUser{Phone: "0500000001", FirstName: "A", LastName: "B", IsAdmin: true}, actor(adm))
	if err != nil {
		t.Fatal(err)
	}
	if u.Status != domain.StatusAwaitingActivation || u.Source != domain.SourceAdmin || !u.IsAdmin || u.IsResident || u.Phone != "+380500000001" {
		t.Errorf("added = %+v", u)
	}
	_, err = e.users.AddUser(ctx, domain.NewUser{Phone: "+380500000001", FirstName: "A", LastName: "B"}, actor(adm))
	wantFields(t, err, map[string]string{"phone": "taken"})
}

func TestPhoneEditRules(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	free := e.mkUser("+380500000001", func(u *domain.User) { u.Status = domain.StatusAwaitingActivation })
	linked := e.mkUser("+380500000002")
	e.link(linked, "222")
	u, err := e.users.Update(ctx, free.ID, domain.UserPatch{Phone: ptr("050 000 00 03")}, actor(adm))
	if err != nil || u.Phone != "+380500000003" {
		t.Errorf("free phone edit = %+v, %v", u, err)
	}
	_, err = e.users.Update(ctx, linked.ID, domain.UserPatch{Phone: ptr("+380500000004")}, actor(adm))
	wantFields(t, err, map[string]string{"phone": "linked"})
	_, err = e.users.Update(ctx, free.ID, domain.UserPatch{Phone: ptr("+380500000002")}, actor(adm))
	wantFields(t, err, map[string]string{"phone": "taken"})
}

func TestLastAdminProtected(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	ch := e.link(adm, "999")
	if _, err := e.users.Update(ctx, adm.ID, domain.UserPatch{IsAdmin: ptr(false)}, actor(adm)); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("revoke err = %v", err)
	}
	if err := e.users.Disable(ctx, adm.ID, actor(adm)); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("disable err = %v", err)
	}
	if err := e.users.UnlinkChannel(ctx, adm.ID, ch.ID, actor(adm)); !errors.Is(err, domain.ErrLastAdmin) {
		t.Errorf("unlink err = %v", err)
	}
	second := e.mkUser("+380500000098", admin)
	if _, err := e.users.Update(ctx, adm.ID, domain.UserPatch{IsAdmin: ptr(false)}, actor(second)); err != nil {
		t.Errorf("revoke with a second admin = %v", err)
	}
	if e.auditCount(domain.ActUserAdminRevoked) != 1 {
		t.Error("no admin_revoked audit")
	}
}

func TestSessionsEndedOnSensitiveChanges(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	e.link(adm, "999")
	steps := []struct {
		name string
		do   func(u domain.User, ch domain.Channel) error
	}{
		{"admin revoke", func(u domain.User, _ domain.Channel) error {
			_, err := e.users.Update(ctx, u.ID, domain.UserPatch{IsAdmin: ptr(false)}, actor(adm))
			return err
		}},
		{"disable", func(u domain.User, _ domain.Channel) error { return e.users.Disable(ctx, u.ID, actor(adm)) }},
		{"unlink", func(u domain.User, ch domain.Channel) error { return e.users.UnlinkChannel(ctx, u.ID, ch.ID, actor(adm)) }},
	}
	for i, s := range steps {
		u := e.mkUser("+38050000001"+string(rune('0'+i)), admin)
		ch := e.link(u, "c"+string(rune('0'+i)))
		e.session(u)
		if err := s.do(u, ch); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if n := e.sessions(u); n != 0 {
			t.Errorf("%s left %d sessions", s.name, n)
		}
	}
	// Phone change (only possible without a channel) also ends sessions.
	u := e.mkUser("+380500000020", func(u *domain.User) { u.Status = domain.StatusAwaitingActivation })
	e.session(u)
	if _, err := e.users.Update(ctx, u.ID, domain.UserPatch{Phone: ptr("+380500000021")}, actor(adm)); err != nil {
		t.Fatal(err)
	}
	if e.sessions(u) != 0 {
		t.Error("phone change kept sessions")
	}
}

func TestUnlinkReturnsToAwaitingActivation(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	u := e.mkUser("+380500000001")
	ch := e.link(u, "111")
	if err := e.users.UnlinkChannel(ctx, u.ID, ch.ID, actor(adm)); err != nil {
		t.Fatal(err)
	}
	if got := e.get(u.ID); got.Status != domain.StatusAwaitingActivation {
		t.Errorf("status = %s", got.Status)
	}
	if err := e.users.UnlinkChannel(ctx, u.ID, ch.ID, actor(adm)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unlink twice err = %v", err)
	}
}

func TestEnableRestoresStatus(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	withCh := e.mkUser("+380500000001")
	e.link(withCh, "111")
	noCh := e.mkUser("+380500000002", func(u *domain.User) { u.Status = domain.StatusAwaitingActivation })
	for _, u := range []domain.User{withCh, noCh} {
		if err := e.users.Disable(ctx, u.ID, actor(adm)); err != nil {
			t.Fatal(err)
		}
		if err := e.users.Enable(ctx, u.ID, actor(adm)); err != nil {
			t.Fatal(err)
		}
	}
	if e.get(withCh.ID).Status != domain.StatusActive || e.get(noCh.ID).Status != domain.StatusAwaitingActivation {
		t.Errorf("statuses %s / %s", e.get(withCh.ID).Status, e.get(noCh.ID).Status)
	}
}

func TestResidencyRules(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	u := e.mkUser("+380500000001")
	_, err := e.users.AddResidency(ctx, u.ID, domain.Residency{HouseID: e.house.ID, Apartment: 11, StatusID: e.owner.ID}, actor(adm))
	wantFields(t, err, map[string]string{"apartment": "out_of_range"})
	r, err := e.users.AddResidency(ctx, u.ID, domain.Residency{HouseID: e.house.ID, Apartment: 4, StatusID: e.owner.ID}, actor(adm))
	if err != nil || r.UserID != u.ID {
		t.Fatalf("AddResidency = %+v, %v", r, err)
	}
	_, err = e.users.AddResidency(ctx, u.ID, domain.Residency{HouseID: e.house.ID, Apartment: 4, StatusID: e.owner.ID}, actor(adm))
	wantFields(t, err, map[string]string{"apartment": "taken"})
	r.Apartment = 5
	if err := e.users.UpdateResidency(ctx, r, actor(adm)); err != nil {
		t.Fatal(err)
	}
	if err := e.users.RemoveResidency(ctx, r.ID, actor(adm)); err != nil {
		t.Fatal(err)
	}
}

func TestCardHasAuditNewestFirst(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	_ = e.users.Apply(ctx, e.application("+380500000001"), public)
	u, _ := e.r.Users.GetByPhone(ctx, "+380500000001")
	e.clock.Advance(1)
	_ = e.users.Approve(ctx, u.ID, actor(adm))
	card, err := e.users.Card(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(card.Audit) != 2 || card.Audit[0].Action != domain.ActUserApproved || len(card.Residencies) != 1 {
		t.Errorf("card = %+v", card)
	}
}
