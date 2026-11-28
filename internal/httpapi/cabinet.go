package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Manifest values that match the web app's Mantine theme (teal, shade 7).
const (
	manifestThemeColor      = "#0ca678"
	manifestBackgroundColor = "#ffffff"
	manifestFallbackName    = "Homelander"
	manifestShortNameMax    = 12
)

// RegisterCabinet adds what the resident cabinet needs: the login link
// redeem endpoint, the cabinet's "My home" data and the PWA manifest.
func RegisterCabinet(s *Server, api API) {
	h := handlerCtx{s, api}
	m := s.API()
	m.HandleFunc("POST /api/v1/auth/link", h.authLink)
	m.Handle("GET /api/v1/me/home", RequireUser(http.HandlerFunc(h.myHome)))
	s.Mount("/site.webmanifest", http.HandlerFunc(h.manifest))
}

// authLink spends a login link from the bot and opens a session. Only this
// POST from our page spends it: the /login/link page itself is a plain SPA
// route, so a link preview fetched by the messenger changes nothing.
func (h handlerCtx) authLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
		// Switch confirms signing out of the current account: without it a
		// browser signed in as someone else gets 409 link_other_user.
		Switch bool `json:"switch"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	u, token, next, err := h.api.Auth.UseLoginLink(r.Context(), in.Token, r.UserAgent(), h.actor(r), in.Switch)
	if err != nil {
		h.fail(w, err)
		return
	}
	// The browser's old session (the same user, or one the person chose to
	// leave) is closed rather than left behind. The link is spent and the new
	// session open by now, so a failure here must not fail the login: the old
	// session only lingers until it expires.
	if old := sessionToken(r); old != "" {
		if err := h.api.Auth.Logout(r.Context(), old, h.actor(r)); err != nil {
			h.s.deps.Log.Warn("previous session not closed after a login link", "user", u.ID, "err", err)
		}
	}
	SetSessionCookie(w, token, h.api.Cfg.SessionMaxTTL)
	me, err := h.meDTO(r, u)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"user": me, "next": next})
}

type homeDTO struct {
	ID          int64                `json:"id"`
	HouseName   string               `json:"houseName"`
	Address     string               `json:"address"`
	Floors      *int                 `json:"floors"`
	Apartment   int                  `json:"apartment"`
	StatusNames domain.LocalizedText `json:"statusNames"`
}

// myHome returns the signed-in user's apartments with their houses, and
// whether a messenger is linked: the cabinet's "My home" section.
func (h handlerCtx) myHome(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r)
	card, err := h.api.Users.Card(r.Context(), u.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	homes := make([]homeDTO, 0, len(card.Residencies))
	for _, rs := range card.Residencies {
		house := n.houses[rs.HouseID]
		names := n.statuses[rs.StatusID].Names
		if names == nil {
			names = domain.LocalizedText{}
		}
		homes = append(homes, homeDTO{ID: rs.ID, HouseName: house.Name, Address: house.Address, Floors: house.Floors,
			Apartment: rs.Apartment, StatusNames: names})
	}
	linked := false
	for _, c := range card.Channels {
		linked = linked || c.Type == h.api.Messenger.Type()
	}
	WriteJSON(w, http.StatusOK, map[string]any{"homes": homes, "telegramLinked": linked})
}

type manifestIcon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

// manifest serves the PWA manifest named after the community, in the site's
// default language.
func (h handlerCtx) manifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	st, err := h.api.Catalog.Site(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	lang := st.DefaultLocale
	if lang == "" {
		lang = h.api.Cfg.DefaultLocale
	}
	name := communityName(st.CommunityName, lang, h.api.Cfg.DefaultLocale)
	body, err := json.Marshal(map[string]any{
		"name": name, "short_name": shortName(name), "lang": lang,
		"start_url": "/my", "scope": "/", "display": "standalone",
		"theme_color": manifestThemeColor, "background_color": manifestBackgroundColor,
		"icons": []manifestIcon{
			{"/icon-192.png", "192x192", "image/png"},
			{"/icon-512.png", "512x512", "image/png"},
		},
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(body)
}

// communityName picks the name in lang, then in fallback, then any filled
// value, then the product name.
func communityName(t domain.LocalizedText, lang, fallback string) string {
	for _, l := range []string{lang, fallback} {
		if v := strings.TrimSpace(t[l]); v != "" {
			return v
		}
	}
	for _, v := range t {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return manifestFallbackName
}

// shortName is the name under the home-screen icon: the whole name when it
// fits in 12 characters, else its first word when that fits, else its first
// 12 characters.
func shortName(name string) string {
	if utf8.RuneCountInString(name) <= manifestShortNameMax {
		return name
	}
	if f := strings.Fields(name); len(f) > 0 && utf8.RuneCountInString(f[0]) <= manifestShortNameMax {
		return f[0]
	}
	return strings.TrimSpace(string([]rune(name)[:manifestShortNameMax]))
}
