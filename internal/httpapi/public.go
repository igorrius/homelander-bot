package httpapi

import (
	"net/http"
	"strconv"

	"github.com/igorrius/homelander-bot/internal/config"
	"github.com/igorrius/homelander-bot/internal/domain"
	"github.com/igorrius/homelander-bot/internal/ratelimit"
)

// API bundles what the JSON handlers need.
type API struct {
	Users     *domain.UserService
	Catalog   *domain.CatalogService
	Data      *domain.DataService
	Pages     *domain.PageService
	Auth      *domain.AuthService
	Chats     *domain.ChatService
	Labels    *domain.LabelService
	Notifier  *domain.Notifier
	Modules   *domain.ModuleService
	Feed      *domain.FeedService
	MCPAuth   *domain.McpAuthService
	Limits    *ratelimit.Limiter // nil disables the OAuth endpoint limits
	Messenger domain.Messenger
	Cfg       config.Config
}

type handlerCtx struct {
	s   *Server
	api API
}

func (h handlerCtx) actor(r *http.Request) domain.Actor {
	a := domain.Actor{IP: ClientIP(r, h.api.Cfg.TrustProxy)}
	if u, ok := CurrentUser(r); ok {
		a.UserID = &u.ID
	}
	return a
}

func (h handlerCtx) fail(w http.ResponseWriter, err error) { writeDomainError(w, h.s.deps.Log, err) }

// RegisterPublic adds the routes that need no session or only a signed-in user.
func RegisterPublic(s *Server, api API) {
	h := handlerCtx{s, api}
	m := s.API()
	m.HandleFunc("GET /api/v1/site", h.site)
	m.HandleFunc("GET /api/v1/register/options", h.registerOptions)
	m.HandleFunc("POST /api/v1/register", h.register)
	m.HandleFunc("POST /api/v1/register/prefill", h.prefill)
	m.HandleFunc("POST /api/v1/auth/code", h.authCode)
	m.HandleFunc("POST /api/v1/auth/verify", h.authVerify)
	m.HandleFunc("POST /api/v1/auth/logout", h.logout)
	m.Handle("GET /api/v1/me", RequireUser(http.HandlerFunc(h.me)))
	m.Handle("GET /api/v1/me/data", RequireUser(http.HandlerFunc(h.myData)))
	m.HandleFunc("GET /api/v1/assets/{id}", h.asset)
	m.HandleFunc("GET /api/v1/public/data", h.publicData)
	m.HandleFunc("GET /api/v1/public/data/{key}", h.publicDataOne)
}

func (h handlerCtx) site(w http.ResponseWriter, r *http.Request) {
	st, err := h.api.Catalog.Site(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writeSite(w, r, st)
}

func (h handlerCtx) registerOptions(w http.ResponseWriter, r *http.Request) {
	houses, err := h.api.Catalog.Houses(r.Context(), false)
	if err != nil {
		h.fail(w, err)
		return
	}
	statuses, err := h.api.Catalog.Statuses(r.Context(), false)
	if err != nil {
		h.fail(w, err)
		return
	}
	type houseOpt struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		ApartmentFrom int    `json:"apartmentFrom"`
		ApartmentTo   int    `json:"apartmentTo"`
		Floors        *int   `json:"floors"` // for the built-in page's quarter
	}
	ho := make([]houseOpt, 0, len(houses))
	for _, x := range houses {
		ho = append(ho, houseOpt{x.ID, x.Name, x.ApartmentFrom, x.ApartmentTo, x.Floors})
	}
	so := make([]statusDTO, 0, len(statuses))
	for _, x := range statuses {
		so = append(so, toStatus(x))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"houses": ho, "statuses": so})
}

func (h handlerCtx) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone     string `json:"phone"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		HouseID   int64  `json:"houseId"`
		Apartment int    `json:"apartment"`
		StatusID  int64  `json:"statusId"`
		Consent   bool   `json:"consent"`
		Locale    string `json:"locale"`
		Token     string `json:"token"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	err := h.api.Users.Apply(r.Context(), domain.Application{Phone: in.Phone, FirstName: in.FirstName, LastName: in.LastName,
		HouseID: in.HouseID, Apartment: in.Apartment, StatusID: in.StatusID, Consent: in.Consent, Locale: in.Locale, Token: in.Token}, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "received"})
}

// prefill returns the phone a registration link from the bot proved.
func (h handlerCtx) prefill(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	phone, err := h.api.Users.PrefillRegistration(r.Context(), in.Token)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"phone": phone})
}

func (h handlerCtx) authCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	if err := h.api.Auth.RequestCode(r.Context(), in.Phone, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusAccepted, map[string]string{"status": "sent_if_allowed"})
}

func (h handlerCtx) authVerify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	u, token, err := h.api.Auth.VerifyCode(r.Context(), in.Phone, in.Code, r.UserAgent(), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	SetSessionCookie(w, token, h.api.Cfg.SessionMaxTTL)
	me, err := h.meDTO(r, u)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"user": me})
}

func (h handlerCtx) logout(w http.ResponseWriter, r *http.Request) {
	if tok := sessionToken(r); tok != "" {
		if err := h.api.Auth.Logout(r.Context(), tok, h.actor(r)); err != nil {
			h.fail(w, err)
			return
		}
	}
	ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h handlerCtx) meDTO(r *http.Request, u domain.User) (userDTO, error) {
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		return userDTO{}, err
	}
	card, err := h.api.Users.Card(r.Context(), u.ID)
	if err != nil {
		return userDTO{}, err
	}
	return n.toUser(card.User, card.Residencies), nil
}

func (h handlerCtx) me(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r)
	me, err := h.meDTO(r, u)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, me)
}

func (h handlerCtx) asset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found")
		return
	}
	public, blocked, err := h.api.Data.AssetAccess(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	if public {
		// Short: an item made non-public must stop being served by caches soon.
		h.serveAsset(w, r, id, "public, max-age=60")
		return
	}
	// A signed URL from /me/data: a residents-level image for a page's frame,
	// which sends no cookie. Not for an image only a module that is off uses.
	q := r.URL.Query()
	if !blocked && h.api.Data.ValidAssetSig(id, q.Get("exp"), q.Get("sig")) {
		h.serveAsset(w, r, id, "private, max-age=600")
		return
	}
	WriteError(w, http.StatusNotFound, "not_found")
}

// serveAsset writes a stored file with headers that keep it from running as a page.
func (h handlerCtx) serveAsset(w http.ResponseWriter, r *http.Request, id int64, cache string) {
	a, err := h.api.Catalog.Asset(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Cache-Control", cache)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	_, _ = w.Write(a.Data)
}
