package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// CatalogService manages houses, residency statuses and site settings.
type CatalogService struct {
	R             Repos
	Now           Clock
	Locales       []string // supported locales
	DefaultLocale string   // the locale whose status name is required; defaults to Locales[0]
}

const (
	maxApartments = 2000
	maxFloors     = 200
	maxHouseText  = 200
	maxSiteName   = 200
	maxWelcome    = 2000
	maxHomeItems  = 20
	maxHomeTitle  = 100
	maxImageBytes = 5 << 20
)

var statusCode = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

func (s *CatalogService) audit(ctx context.Context, a Actor, action, target string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: target, Details: details, IP: a.IP})
}

func (s *CatalogService) defaultLocale() string {
	if s.DefaultLocale != "" {
		return s.DefaultLocale
	}
	if len(s.Locales) > 0 {
		return s.Locales[0]
	}
	return "uk"
}

func (s *CatalogService) checkHouse(ctx context.Context, h *House, existing bool) error {
	v := validation{}
	h.Name, h.Address = strings.TrimSpace(h.Name), strings.TrimSpace(h.Address)
	switch {
	case h.Name == "":
		v.add("name", "required")
	case utf8.RuneCountInString(h.Name) > maxNameLen:
		v.add("name", "too_long")
	}
	if utf8.RuneCountInString(h.Address) > maxHouseText {
		v.add("address", "too_long")
	}
	if h.ApartmentFrom < 1 {
		v.add("apartmentFrom", "out_of_range")
	}
	if h.ApartmentTo < h.ApartmentFrom || h.ApartmentTo-h.ApartmentFrom >= maxApartments {
		v.add("apartmentTo", "out_of_range")
	}
	if h.Floors != nil && (*h.Floors < 1 || *h.Floors > maxFloors) {
		v.add("floors", "out_of_range")
	}
	if h.Latitude != nil && (*h.Latitude < -90 || *h.Latitude > 90) {
		v.add("latitude", "out_of_range")
	}
	if h.Longitude != nil && (*h.Longitude < -180 || *h.Longitude > 180) {
		v.add("longitude", "out_of_range")
	}
	if existing && len(v) == 0 {
		old, err := s.R.Houses.Get(ctx, h.ID)
		if err != nil {
			return err
		}
		if h.ApartmentFrom > old.ApartmentFrom {
			if n, err := s.R.Houses.ResidenciesOutside(ctx, h.ID, h.ApartmentFrom, old.ApartmentTo); err != nil {
				return err
			} else if n > 0 {
				v.add("apartmentFrom", "has_residents")
			}
		}
		if h.ApartmentTo < old.ApartmentTo {
			if n, err := s.R.Houses.ResidenciesOutside(ctx, h.ID, old.ApartmentFrom, h.ApartmentTo); err != nil {
				return err
			} else if n > 0 {
				v.add("apartmentTo", "has_residents")
			}
		}
	}
	return v.err()
}

// CreateHouse adds a house.
func (s *CatalogService) CreateHouse(ctx context.Context, h House, actor Actor) (House, error) {
	h.ID = 0
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.checkHouse(ctx, &h, false); err != nil {
			return err
		}
		if err := s.R.Houses.Create(ctx, &h); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActHouseCreated, HouseTarget(h.ID), map[string]any{"name": h.Name})
	})
	return h, err
}

// UpdateHouse changes a house; a range that would leave residents outside is refused.
func (s *CatalogService) UpdateHouse(ctx context.Context, h House, actor Actor) (House, error) {
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.checkHouse(ctx, &h, true); err != nil {
			return err
		}
		if err := s.R.Houses.Update(ctx, &h); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActHouseUpdated, HouseTarget(h.ID), nil)
	})
	return h, err
}

// Houses lists houses.
func (s *CatalogService) Houses(ctx context.Context, includeArchived bool) ([]House, error) {
	return s.R.Houses.List(ctx, includeArchived)
}

// HouseDetail returns a house with its statistics.
func (s *CatalogService) HouseDetail(ctx context.Context, id int64) (House, HouseStats, error) {
	h, err := s.R.Houses.Get(ctx, id)
	if err != nil {
		return h, HouseStats{}, err
	}
	st, err := s.R.Houses.Stats(ctx, id)
	return h, st, err
}

// ApartmentResidents lists who is registered in an apartment.
func (s *CatalogService) ApartmentResidents(ctx context.Context, houseID int64, apartment int) ([]ApartmentResident, error) {
	if _, err := s.R.Houses.Get(ctx, houseID); err != nil {
		return nil, err
	}
	return s.R.Houses.Residents(ctx, houseID, apartment)
}

// checkTexts validates per-locale texts: only supported locales, each within max runes.
func (s *CatalogService) checkTexts(v validation, field string, t LocalizedText, maxLen int) LocalizedText {
	out := LocalizedText{}
	for loc, text := range t {
		if !slices.Contains(s.Locales, loc) {
			v.add(field, "invalid")
			continue
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > maxLen {
			v.add(field, "too_long")
		}
		if text != "" {
			out[loc] = text
		}
	}
	return out
}

// checkHomeItems validates the built-in page cards: at most maxHomeItems,
// valid data keys (not necessarily existing), short titles; a repeated key
// keeps its first entry.
func (s *CatalogService) checkHomeItems(v validation, in []HomeItem) []HomeItem {
	out := []HomeItem{}
	if len(in) > maxHomeItems {
		v.add("homeItems", "too_many")
		return out
	}
	seen := map[string]bool{}
	for _, it := range in {
		if len(it.Key) > maxDataKey || !dataKeyRe.MatchString(it.Key) {
			v.add("homeItems", "invalid")
			continue
		}
		title := s.checkTexts(v, "homeItems", it.Title, maxHomeTitle)
		if seen[it.Key] {
			continue
		}
		seen[it.Key] = true
		out = append(out, HomeItem{Key: it.Key, Title: title})
	}
	return out
}

func (s *CatalogService) checkStatus(st *ResidencyStatus) error {
	v := validation{}
	if !statusCode.MatchString(st.Code) {
		v.add("code", "invalid")
	}
	st.Names = s.checkTexts(v, "names", st.Names, maxNameLen)
	if st.Names[s.defaultLocale()] == "" {
		v.add("names", "required")
	}
	return v.err()
}

func takenCode(err error) error {
	if errors.Is(err, ErrConflict) {
		return &ValidationError{Fields: map[string]string{"code": "taken"}}
	}
	return err
}

// CreateStatus adds a residency status.
func (s *CatalogService) CreateStatus(ctx context.Context, st ResidencyStatus, actor Actor) (ResidencyStatus, error) {
	st.ID = 0
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.checkStatus(&st); err != nil {
			return err
		}
		if err := s.R.Statuses.Create(ctx, &st); err != nil {
			return takenCode(err)
		}
		return s.audit(ctx, actor, ActStatusCreated, StatusTarget(st.ID), map[string]any{"code": st.Code})
	})
	return st, err
}

// UpdateStatus changes a residency status.
func (s *CatalogService) UpdateStatus(ctx context.Context, st ResidencyStatus, actor Actor) (ResidencyStatus, error) {
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.checkStatus(&st); err != nil {
			return err
		}
		if err := s.R.Statuses.Update(ctx, &st); err != nil {
			return takenCode(err)
		}
		return s.audit(ctx, actor, ActStatusUpdated, StatusTarget(st.ID), nil)
	})
	return st, err
}

// Statuses lists residency statuses.
func (s *CatalogService) Statuses(ctx context.Context, includeArchived bool) ([]ResidencyStatus, error) {
	return s.R.Statuses.List(ctx, includeArchived)
}

// Site returns the site settings; an unset default locale is the system's.
func (s *CatalogService) Site(ctx context.Context) (SiteSettings, error) {
	st, err := s.R.Site.Get(ctx)
	if st.DefaultLocale == "" {
		st.DefaultLocale = s.defaultLocale()
	}
	return st, err
}

// SaveSite saves the texts, the default locale and the built-in page cards.
func (s *CatalogService) SaveSite(ctx context.Context, in SiteSettings, actor Actor) (SiteSettings, error) {
	var out SiteSettings
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		v := validation{}
		if !slices.Contains(s.Locales, in.DefaultLocale) {
			v.add("defaultLocale", "invalid")
		}
		name := s.checkTexts(v, "communityName", in.CommunityName, maxSiteName)
		welcome := s.checkTexts(v, "welcomeText", in.WelcomeText, maxWelcome)
		items := s.checkHomeItems(v, in.HomeItems)
		if err := v.err(); err != nil {
			return err
		}
		out = SiteSettings{CommunityName: name, WelcomeText: welcome, DefaultLocale: in.DefaultLocale, HomeItems: items}
		if err := s.R.Site.Save(ctx, out); err != nil {
			return err
		}
		return s.audit(ctx, actor, ActSiteUpdated, SiteTarget, nil)
	})
	return out, err
}

// Asset returns a stored file (a data space image).
func (s *CatalogService) Asset(ctx context.Context, id int64) (Asset, error) {
	return s.R.Site.GetAsset(ctx, id)
}

// dataImageTypes are the image types the data space accepts.
var dataImageTypes = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}

// UploadImage stores an image for the data space; it is served publicly only
// once a public item refers to it. Problems are reported on the "file" field.
func (s *CatalogService) UploadImage(ctx context.Context, data []byte, actor Actor) (Asset, error) {
	if len(data) > maxImageBytes {
		return Asset{}, &ValidationError{Fields: map[string]string{"file": "size"}}
	}
	ct := http.DetectContentType(data)
	if !slices.Contains(dataImageTypes, ct) {
		return Asset{}, &ValidationError{Fields: map[string]string{"file": "type"}}
	}
	sum := sha256.Sum256(data)
	a := Asset{Kind: "data", ContentType: ct, Data: data, SHA256: hex.EncodeToString(sum[:]), UploadedAt: s.Now()}
	if actor.UserID != nil {
		a.UploadedBy = *actor.UserID
	}
	return a, s.R.Site.PutAsset(ctx, &a)
}
