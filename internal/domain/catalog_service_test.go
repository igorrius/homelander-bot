package domain_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/igorrius/homelander-bot/internal/domain"
)

var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestHouseValidation(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	cases := []struct {
		h    domain.House
		want map[string]string
	}{
		{domain.House{Name: "", ApartmentFrom: 1, ApartmentTo: 2}, map[string]string{"name": "required"}},
		{domain.House{Name: "x", ApartmentFrom: 0, ApartmentTo: 2}, map[string]string{"apartmentFrom": "out_of_range"}},
		{domain.House{Name: "x", ApartmentFrom: 5, ApartmentTo: 2}, map[string]string{"apartmentTo": "out_of_range"}},
		{domain.House{Name: "x", ApartmentFrom: 1, ApartmentTo: 2001}, map[string]string{"apartmentTo": "out_of_range"}},
		{domain.House{Name: "x", ApartmentFrom: 1, ApartmentTo: 2, Floors: ptr(0)}, map[string]string{"floors": "out_of_range"}},
		{domain.House{Name: "x", ApartmentFrom: 1, ApartmentTo: 2, Latitude: ptr(91.0)}, map[string]string{"latitude": "out_of_range"}},
		{domain.House{Name: "x", ApartmentFrom: 1, ApartmentTo: 2, Longitude: ptr(-181.0)}, map[string]string{"longitude": "out_of_range"}},
	}
	for _, c := range cases {
		_, err := e.catalog.CreateHouse(ctx, c.h, actor(adm))
		wantFields(t, err, c.want)
	}
	h, err := e.catalog.CreateHouse(ctx, domain.House{Name: " 24 ", ApartmentFrom: 121, ApartmentTo: 240, Floors: ptr(16)}, actor(adm))
	if err != nil || h.ID == 0 || h.Name != "24" {
		t.Fatalf("CreateHouse = %+v, %v", h, err)
	}
}

func TestNarrowRangeWithResidentsRefused(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	_ = e.users.Apply(ctx, e.application("+380500000001"), public) // apartment 3
	h := e.house
	h.ApartmentFrom = 4
	_, err := e.catalog.UpdateHouse(ctx, h, actor(adm))
	wantFields(t, err, map[string]string{"apartmentFrom": "has_residents"})
	h.ApartmentFrom, h.ApartmentTo = 1, 2
	_, err = e.catalog.UpdateHouse(ctx, h, actor(adm))
	wantFields(t, err, map[string]string{"apartmentTo": "has_residents"})
	h.ApartmentTo = 20
	if _, err := e.catalog.UpdateHouse(ctx, h, actor(adm)); err != nil {
		t.Errorf("widening = %v", err)
	}
}

func TestHouseDetail(t *testing.T) {
	e := newEnv(t)
	_ = e.users.Apply(ctx, e.application("+380500000001"), public)
	h, st, err := e.catalog.HouseDetail(ctx, e.house.ID)
	if err != nil || h.ID != e.house.ID || st.People != 1 || st.Apartments[3] != 1 {
		t.Errorf("HouseDetail = %+v %+v %v", h, st, err)
	}
	res, err := e.catalog.ApartmentResidents(ctx, e.house.ID, 3)
	if err != nil || len(res) != 1 {
		t.Errorf("ApartmentResidents = %+v, %v", res, err)
	}
}

func TestStatusValidation(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	_, err := e.catalog.CreateStatus(ctx, domain.ResidencyStatus{Code: "Bad Code", Names: domain.LocalizedText{"uk": "x"}}, actor(adm))
	wantFields(t, err, map[string]string{"code": "invalid"})
	_, err = e.catalog.CreateStatus(ctx, domain.ResidencyStatus{Code: "relative", Names: domain.LocalizedText{"en": "Relative"}}, actor(adm))
	wantFields(t, err, map[string]string{"names": "required"})
	_, err = e.catalog.CreateStatus(ctx, domain.ResidencyStatus{Code: "relative", Names: domain.LocalizedText{"uk": "Родич", "de": "x"}}, actor(adm))
	wantFields(t, err, map[string]string{"names": "invalid"})
	_, err = e.catalog.CreateStatus(ctx, domain.ResidencyStatus{Code: "owner", Names: domain.LocalizedText{"uk": "x"}}, actor(adm))
	wantFields(t, err, map[string]string{"code": "taken"})
	st, err := e.catalog.CreateStatus(ctx, domain.ResidencyStatus{Code: "relative", Names: domain.LocalizedText{"uk": "Родич", "en": "Relative"}}, actor(adm))
	if err != nil || st.ID == 0 {
		t.Fatalf("CreateStatus = %+v, %v", st, err)
	}
}

func TestSiteValidation(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	_, err := e.catalog.SaveSite(ctx, domain.SiteSettings{DefaultLocale: "de"}, actor(adm))
	wantFields(t, err, map[string]string{"defaultLocale": "invalid"})
	_, err = e.catalog.SaveSite(ctx, domain.SiteSettings{DefaultLocale: "uk", CommunityName: domain.LocalizedText{"de": "x"}}, actor(adm))
	wantFields(t, err, map[string]string{"communityName": "invalid"})
	_, err = e.catalog.SaveSite(ctx, domain.SiteSettings{DefaultLocale: "uk", WelcomeText: domain.LocalizedText{"uk": string(bytes.Repeat([]byte("я"), 2001))}}, actor(adm))
	wantFields(t, err, map[string]string{"welcomeText": "too_long"})
	got, err := e.catalog.SaveSite(ctx, domain.SiteSettings{DefaultLocale: "en", CommunityName: domain.LocalizedText{"uk": "Дім"}}, actor(adm))
	if err != nil || got.DefaultLocale != "en" || got.CommunityName["uk"] != "Дім" {
		t.Errorf("SaveSite = %+v, %v", got, err)
	}
}

func TestSaveSiteKeepsPicture(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	a, err := e.catalog.SetPicture(ctx, pngHeader, actor(adm))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := e.catalog.SaveSite(ctx, domain.SiteSettings{DefaultLocale: "uk"}, actor(adm))
	if got.PictureAssetID == nil || *got.PictureAssetID != a.ID {
		t.Errorf("SaveSite dropped the picture: %+v", got)
	}
}

func TestPictureTypes(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	a, err := e.catalog.SetPicture(ctx, pngHeader, actor(adm))
	if err != nil || a.ContentType != "image/png" || a.SHA256 == "" {
		t.Fatalf("PNG = %+v, %v", a, err)
	}
	_, err = e.catalog.SetPicture(ctx, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), actor(adm))
	wantFields(t, err, map[string]string{"picture": "type"})
	big := append(append([]byte{}, pngHeader...), make([]byte, 5<<20)...)
	_, err = e.catalog.SetPicture(ctx, big, actor(adm))
	wantFields(t, err, map[string]string{"picture": "size"})
}

func TestPictureReplaceDeletesOld(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	first, _ := e.catalog.SetPicture(ctx, pngHeader, actor(adm))
	second, _ := e.catalog.SetPicture(ctx, pngHeader, actor(adm))
	if _, err := e.r.Site.GetAsset(ctx, first.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("old asset err = %v", err)
	}
	site, _ := e.catalog.Site(ctx)
	if site.PictureAssetID == nil || *site.PictureAssetID != second.ID {
		t.Errorf("site picture = %v", site.PictureAssetID)
	}
	if err := e.catalog.RemovePicture(ctx, actor(adm)); err != nil {
		t.Fatal(err)
	}
	site, _ = e.catalog.Site(ctx)
	if site.PictureAssetID != nil {
		t.Error("picture not removed")
	}
	if _, err := e.r.Site.GetAsset(ctx, second.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("removed asset err = %v", err)
	}
}

func TestCatalogAudit(t *testing.T) {
	e := newEnv(t)
	adm := e.mkUser("+380500000099", admin)
	h, _ := e.catalog.CreateHouse(ctx, domain.House{Name: "2", ApartmentFrom: 1, ApartmentTo: 5}, actor(adm))
	_, _ = e.catalog.UpdateHouse(ctx, h, actor(adm))
	_, _ = e.catalog.SaveSite(ctx, domain.SiteSettings{DefaultLocale: "uk"}, actor(adm))
	for _, a := range []string{domain.ActHouseCreated, domain.ActHouseUpdated, domain.ActSiteUpdated} {
		if e.auditCount(a) != 1 {
			t.Errorf("audit %s missing", a)
		}
	}
}
