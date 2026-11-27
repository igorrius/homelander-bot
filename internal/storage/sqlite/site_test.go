package sqlite_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite"
	"github.com/igorrius/homelander-bot/internal/storage/sqlite/sqlitetest"
)

func TestSiteDefaultsWhenUnset(t *testing.T) {
	site := sqlite.Site{DB: sqlitetest.New(t)}
	got, err := site.Get(ctx)
	if err != nil || got.CommunityName == nil || got.WelcomeText == nil || got.PictureAssetID != nil || got.DefaultLocale != "" {
		t.Errorf("Get = %+v, %v", got, err)
	}
}

func TestSiteRoundTrip(t *testing.T) {
	site := sqlite.Site{DB: sqlitetest.New(t)}
	id := int64(7)
	want := domain.SiteSettings{
		CommunityName: domain.LocalizedText{"uk": "Наш дім", "en": "Our home"},
		WelcomeText:   domain.LocalizedText{"uk": "Вітаємо", "en": "Welcome"},
		DefaultLocale: "uk", PictureAssetID: &id,
		HomeItems: []domain.HomeItem{{Key: "contacts", Title: domain.LocalizedText{"uk": "Контакти"}}, {Key: "house.address", Title: domain.LocalizedText{}}},
	}
	if err := site.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := site.Get(ctx)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Get = %+v, %v", got, err)
	}
	want.DefaultLocale = "en"
	if err := site.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	if got, _ := site.Get(ctx); got.DefaultLocale != "en" {
		t.Errorf("second save lost: %+v", got)
	}
}

func TestAssetRoundTrip(t *testing.T) {
	site := sqlite.Site{DB: sqlitetest.New(t)}
	a := domain.Asset{Kind: "hero", ContentType: "image/png", Data: []byte{0x89, 'P', 'N', 'G'}, SHA256: "abc", UploadedBy: 1, UploadedAt: t0}
	if err := site.PutAsset(ctx, &a); err != nil || a.ID == 0 {
		t.Fatalf("PutAsset = %+v, %v", a, err)
	}
	got, err := site.GetAsset(ctx, a.ID)
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("GetAsset = %+v, %v", got, err)
	}
	_ = site.DeleteAsset(ctx, a.ID)
	if _, err := site.GetAsset(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("after delete err = %v", err)
	}
}

func TestAuditListByTargetUser(t *testing.T) {
	audit := sqlite.Audit{DB: sqlitetest.New(t)}
	actor := int64(1)
	for i, e := range []domain.AuditEntry{
		{At: t0, ActorUserID: &actor, Action: domain.ActUserApproved, Target: domain.UserTarget(5), Details: map[string]any{"n": 1}},
		{At: t0.Add(1), Action: domain.ActHouseCreated, Target: domain.HouseTarget(5)},
		{At: t0.Add(2), Action: domain.ActUserDisabled, Target: domain.UserTarget(5), IP: "1.2.3.4"},
		{At: t0.Add(3), Action: domain.ActUserDisabled, Target: domain.UserTarget(55)},
	} {
		if err := audit.Add(ctx, e); err != nil {
			t.Fatal(i, err)
		}
	}
	uid := int64(5)
	got, total, err := audit.List(ctx, domain.AuditFilter{TargetUserID: &uid, Limit: 10})
	if err != nil || total != 2 || len(got) != 2 {
		t.Fatalf("List = %+v total %d, %v", got, total, err)
	}
	if got[0].Action != domain.ActUserDisabled || got[0].IP != "1.2.3.4" || got[1].ActorUserID == nil || got[1].Details["n"] != float64(1) {
		t.Errorf("entries = %+v", got)
	}
	all, total, _ := audit.List(ctx, domain.AuditFilter{Limit: 2})
	if total != 4 || len(all) != 2 || all[0].Target != domain.UserTarget(55) {
		t.Errorf("all = %+v total %d", all, total)
	}
}
