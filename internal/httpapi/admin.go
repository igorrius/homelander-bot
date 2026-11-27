package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
	maxUpload       = 5<<20 + 1<<10
)

// RegisterAdmin adds the /api/v1/admin routes, all behind RequireAdmin.
func RegisterAdmin(s *Server, api API) {
	h := handlerCtx{s, api}
	routes := map[string]http.HandlerFunc{
		"GET /api/v1/admin/summary":                            h.summary,
		"GET /api/v1/admin/users":                              h.listUsers,
		"POST /api/v1/admin/users":                             h.addUser,
		"GET /api/v1/admin/users/{id}":                         h.userCard,
		"PATCH /api/v1/admin/users/{id}":                       h.patchUser,
		"POST /api/v1/admin/users/{id}/approve":                h.userAction(api.Users.Approve),
		"POST /api/v1/admin/users/{id}/reject":                 h.userAction(api.Users.Reject),
		"POST /api/v1/admin/users/{id}/disable":                h.userAction(api.Users.Disable),
		"POST /api/v1/admin/users/{id}/enable":                 h.userAction(api.Users.Enable),
		"DELETE /api/v1/admin/users/{id}/channels/{channelId}": h.unlink,
		"POST /api/v1/admin/users/{id}/residencies":            h.addResidency,
		"PATCH /api/v1/admin/residencies/{id}":                 h.patchResidency,
		"DELETE /api/v1/admin/residencies/{id}":                h.deleteResidency,
		"GET /api/v1/admin/houses":                             h.listHouses,
		"POST /api/v1/admin/houses":                            h.createHouse,
		"GET /api/v1/admin/houses/{id}":                        h.houseDetail,
		"PATCH /api/v1/admin/houses/{id}":                      h.patchHouse,
		"GET /api/v1/admin/houses/{id}/apartments/{n}":         h.apartment,
		"GET /api/v1/admin/statuses":                           h.listStatuses,
		"POST /api/v1/admin/statuses":                          h.createStatus,
		"PATCH /api/v1/admin/statuses/{id}":                    h.patchStatus,
		"GET /api/v1/admin/site":                               h.adminSite,
		"PUT /api/v1/admin/site":                               h.saveSite,
		"PUT /api/v1/admin/site/picture":                       h.uploadPicture,
		"DELETE /api/v1/admin/site/picture":                    h.deletePicture,
		"GET /api/v1/admin/audit":                              h.audit,
	}
	registerChats(h, routes)
	registerData(h, routes)
	for pattern, fn := range routes {
		s.API().Handle(pattern, RequireAdmin(fn))
	}
}

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.ErrNotFound
	}
	return id, nil
}

func queryBool(r *http.Request, key string, v validationFields) *bool {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		v[key] = "invalid"
		return nil
	}
	return &b
}

type validationFields map[string]string

func (v validationFields) err() error {
	if len(v) == 0 {
		return nil
	}
	return &domain.ValidationError{Fields: v}
}

// page reads page (1…) and pageSize (≤ 100, default 25) into limit/offset.
func page(r *http.Request) (limit, offset int) {
	q := r.URL.Query()
	size, err := strconv.Atoi(q.Get("pageSize"))
	if err != nil || size <= 0 {
		size = defaultPageSize
	}
	size = min(size, maxPageSize)
	p, err := strconv.Atoi(q.Get("page"))
	if err != nil || p < 1 {
		p = 1
	}
	return size, (p - 1) * size
}

func (h handlerCtx) summary(w http.ResponseWriter, r *http.Request) {
	out := map[string]int{}
	for key, st := range map[string]domain.UserStatus{
		"pendingApplications": domain.StatusPendingApproval,
		"awaitingActivation":  domain.StatusAwaitingActivation,
		"activeUsers":         domain.StatusActive,
		"disabledUsers":       domain.StatusDisabled,
		"rejectedUsers":       domain.StatusRejected,
	} {
		n, err := h.api.Users.CountByStatus(r.Context(), st)
		if err != nil {
			h.fail(w, err)
			return
		}
		out[key] = n
	}
	chats, err := h.api.Chats.NewCount(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	out["newChats"] = chats
	WriteJSON(w, http.StatusOK, out)
}

// adminUser builds the admin view of one user.
func (h handlerCtx) adminUser(ctx context.Context, n *namer, u domain.User) (userDTO, error) {
	card, err := h.api.Users.Card(ctx, u.ID)
	if err != nil {
		return userDTO{}, err
	}
	return n.toAdminUser(u, card.Residencies, hasType(card.Channels, h.api.Messenger.Type())), nil
}

func hasType(chans []domain.Channel, typ string) bool {
	for _, c := range chans {
		if c.Type == typ {
			return true
		}
	}
	return false
}

func (h handlerCtx) listUsers(w http.ResponseWriter, r *http.Request) {
	v := validationFields{}
	q := r.URL.Query()
	f := domain.UserFilter{Query: q.Get("q"), IsAdmin: queryBool(r, "isAdmin", v), IsResident: queryBool(r, "isResident", v)}
	f.Limit, f.Offset = page(r)
	if raw := q.Get("houseId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			f.HouseID = &id
		} else {
			v["houseId"] = "invalid"
		}
	}
	if raw := q.Get("status"); raw != "" {
		st := domain.UserStatus(raw)
		switch st {
		case domain.StatusPendingApproval, domain.StatusAwaitingActivation, domain.StatusActive, domain.StatusRejected, domain.StatusDisabled:
			f.Status = &st
		default:
			v["status"] = "invalid"
		}
	}
	switch q.Get("sort") {
	case "", "name":
	case "newest":
		f.NewestFirst = true
	default:
		v["sort"] = "invalid"
	}
	if err := v.err(); err != nil {
		h.fail(w, err)
		return
	}
	users, total, err := h.api.Users.List(r.Context(), f)
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		d, err := h.adminUser(r.Context(), n, u)
		if err != nil {
			h.fail(w, err)
			return
		}
		out = append(out, d)
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out, "total": total})
}

type channelDTO struct {
	ID       int64     `json:"id"`
	Type     string    `json:"type"`
	Username string    `json:"username"`
	LinkedAt time.Time `json:"linkedAt"`
}

type auditDTO struct {
	ID          int64                `json:"id"`
	At          time.Time            `json:"at"`
	Action      string               `json:"action"`
	Target      string               `json:"target"`
	TargetName  string               `json:"targetName"`
	TargetNames domain.LocalizedText `json:"targetNames"`
	ActorUserID *int64               `json:"actorUserId"`
	ActorName   string               `json:"actorName"`
	Details     map[string]any       `json:"details"`
	IP          string               `json:"ip"`
}

// toAudit adds the names of the people, houses and statuses the entries mention.
// A name that cannot be resolved stays empty; the client then shows the raw target.
func toAudit(ctx context.Context, n *namer, es []domain.AuditEntry) []auditDTO {
	out := make([]auditDTO, 0, len(es))
	for _, e := range es {
		d := auditDTO{ID: e.ID, At: e.At, Action: e.Action, Target: e.Target, ActorUserID: e.ActorUserID, Details: e.Details, IP: e.IP}
		if e.ActorUserID != nil {
			d.ActorName = n.userName(ctx, *e.ActorUserID)
		}
		kind, raw, _ := strings.Cut(e.Target, ":")
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			switch kind {
			case "user":
				d.TargetName = n.userName(ctx, id)
			case "house":
				d.TargetName = n.houses[id].Name
			case "status":
				d.TargetNames = n.statuses[id].Names
			case "chat":
				n.chatNames(ctx)
				d.TargetName = n.chats[id]
			case "topic":
				n.chatNames(ctx)
				d.TargetName = n.topics[id]
			case "label":
				n.chatNames(ctx)
				d.TargetName = n.labels[id]
			}
		}
		out = append(out, d)
	}
	return out
}

// writeCard answers with the user card of id.
func (h handlerCtx) writeCard(w http.ResponseWriter, r *http.Request, id int64, status int) {
	card, err := h.api.Users.Card(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	chans := make([]channelDTO, 0, len(card.Channels))
	for _, c := range card.Channels {
		chans = append(chans, channelDTO{c.ID, c.Type, c.Username, c.LinkedAt})
	}
	WriteJSON(w, status, map[string]any{
		"user":        n.toAdminUser(card.User, card.Residencies, hasType(card.Channels, h.api.Messenger.Type())),
		"channels":    chans,
		"residencies": n.residencies(card.Residencies),
		"audit":       toAudit(r.Context(), n, card.Audit),
	})
}

func (h handlerCtx) addUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone      string `json:"phone"`
		FirstName  string `json:"firstName"`
		LastName   string `json:"lastName"`
		IsResident bool   `json:"isResident"`
		IsAdmin    bool   `json:"isAdmin"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	u, err := h.api.Users.AddUser(r.Context(), domain.NewUser{Phone: in.Phone, FirstName: in.FirstName, LastName: in.LastName,
		IsResident: in.IsResident, IsAdmin: in.IsAdmin}, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writeCard(w, r, u.ID, http.StatusCreated)
}

func (h handlerCtx) userCard(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writeCard(w, r, id, http.StatusOK)
}

func (h handlerCtx) patchUser(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in struct {
		FirstName  *string `json:"firstName"`
		LastName   *string `json:"lastName"`
		Phone      *string `json:"phone"`
		Locale     *string `json:"locale"`
		IsResident *bool   `json:"isResident"`
		IsAdmin    *bool   `json:"isAdmin"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	if _, err := h.api.Users.Update(r.Context(), id, domain.UserPatch(in), h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.writeCard(w, r, id, http.StatusOK)
}

func (h handlerCtx) userAction(fn func(context.Context, int64, domain.Actor) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r, "id")
		if err != nil {
			h.fail(w, err)
			return
		}
		if err := fn(r.Context(), id, h.actor(r)); err != nil {
			h.fail(w, err)
			return
		}
		h.writeCard(w, r, id, http.StatusOK)
	}
}

func (h handlerCtx) unlink(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	chID, err := pathID(r, "channelId")
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.api.Users.UnlinkChannel(r.Context(), id, chID, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.writeCard(w, r, id, http.StatusOK)
}

type residencyIn struct {
	HouseID   int64 `json:"houseId"`
	Apartment int   `json:"apartment"`
	StatusID  int64 `json:"statusId"`
}

func (h handlerCtx) addResidency(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in residencyIn
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	if _, err := h.api.Users.AddResidency(r.Context(), id, domain.Residency{HouseID: in.HouseID, Apartment: in.Apartment, StatusID: in.StatusID}, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.writeCard(w, r, id, http.StatusCreated)
}

func (h handlerCtx) patchResidency(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in residencyIn
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	owner, err := h.api.Users.ResidencyOwner(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.api.Users.UpdateResidency(r.Context(), domain.Residency{ID: id, HouseID: in.HouseID, Apartment: in.Apartment, StatusID: in.StatusID}, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.writeCard(w, r, owner, http.StatusOK)
}

func (h handlerCtx) deleteResidency(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	owner, err := h.api.Users.ResidencyOwner(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.api.Users.RemoveResidency(r.Context(), id, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.writeCard(w, r, owner, http.StatusOK)
}

func (h handlerCtx) listHouses(w http.ResponseWriter, r *http.Request) {
	hs, err := h.api.Catalog.Houses(r.Context(), r.URL.Query().Get("archived") == "true")
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]houseDTO, 0, len(hs))
	for _, x := range hs {
		out = append(out, toHouse(x))
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h handlerCtx) createHouse(w http.ResponseWriter, r *http.Request) {
	var in houseDTO
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	got, err := h.api.Catalog.CreateHouse(r.Context(), in.domain(), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, toHouse(got))
}

func (h handlerCtx) patchHouse(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in houseDTO
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	in.ID = id
	got, err := h.api.Catalog.UpdateHouse(r.Context(), in.domain(), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, toHouse(got))
}

func (h handlerCtx) houseDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	house, st, err := h.api.Catalog.HouseDetail(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	byStatus := map[string]int{}
	for k, v := range st.ByResidencyStatus {
		byStatus[strconv.FormatInt(k, 10)] = v
	}
	apts := map[string]int{}
	for k, v := range st.Apartments {
		apts[strconv.Itoa(k)] = v
	}
	WriteJSON(w, http.StatusOK, map[string]any{"house": toHouse(house), "stats": map[string]any{
		"apartmentsTotal": st.ApartmentsTotal, "apartmentsWithResidents": st.ApartmentsWithResidents, "people": st.People,
		"byResidencyStatus": byStatus, "byUserStatus": st.ByUserStatus, "apartments": apts,
	}})
}

func (h handlerCtx) apartment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	num, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		h.fail(w, domain.ErrNotFound)
		return
	}
	res, err := h.api.Catalog.ApartmentResidents(r.Context(), id, num)
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	type item struct {
		User      userDTO      `json:"user"`
		Residency residencyDTO `json:"residency"`
	}
	out := make([]item, 0, len(res))
	for _, ar := range res {
		u, err := h.adminUser(r.Context(), n, ar.User)
		if err != nil {
			h.fail(w, err)
			return
		}
		out = append(out, item{u, n.residencies([]domain.Residency{ar.Residency})[0]})
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h handlerCtx) listStatuses(w http.ResponseWriter, r *http.Request) {
	ss, err := h.api.Catalog.Statuses(r.Context(), true)
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]statusDTO, 0, len(ss))
	for _, x := range ss {
		out = append(out, toStatus(x))
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h handlerCtx) createStatus(w http.ResponseWriter, r *http.Request) {
	var in statusDTO
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	got, err := h.api.Catalog.CreateStatus(r.Context(), domain.ResidencyStatus{Code: in.Code, Names: in.Names, SortOrder: in.SortOrder, Archived: in.Archived}, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, toStatus(got))
}

func (h handlerCtx) patchStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in statusDTO
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	got, err := h.api.Catalog.UpdateStatus(r.Context(), domain.ResidencyStatus{ID: id, Code: in.Code, Names: in.Names, SortOrder: in.SortOrder, Archived: in.Archived}, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, toStatus(got))
}

func (h handlerCtx) writeSite(w http.ResponseWriter, st domain.SiteSettings) {
	WriteJSON(w, http.StatusOK, map[string]any{"communityName": st.CommunityName, "welcomeText": st.WelcomeText,
		"defaultLocale": st.DefaultLocale, "pictureUrl": assetURL(st.PictureAssetID),
		"registerUrl": h.api.Cfg.PublicURL.String() + "/register", "botUrl": h.api.Messenger.BotURL()})
}

func (h handlerCtx) adminSite(w http.ResponseWriter, r *http.Request) {
	st, err := h.api.Catalog.Site(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writeSite(w, st)
}

func (h handlerCtx) saveSite(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CommunityName domain.LocalizedText `json:"communityName"`
		WelcomeText   domain.LocalizedText `json:"welcomeText"`
		DefaultLocale string               `json:"defaultLocale"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	st, err := h.api.Catalog.SaveSite(r.Context(), domain.SiteSettings{CommunityName: in.CommunityName, WelcomeText: in.WelcomeText, DefaultLocale: in.DefaultLocale}, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writeSite(w, st)
}

// readUpload reads the multipart file field, capped at maxUpload; problems are
// reported on that field and ok is false.
func (h handlerCtx) readUpload(w http.ResponseWriter, r *http.Request, field string) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+64<<10)
	if err := r.ParseMultipartForm(maxUpload); err != nil { //nolint:gosec // G120: the body is capped by MaxBytesReader above
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			h.fail(w, &domain.ValidationError{Fields: map[string]string{field: "size"}})
			return nil, false
		}
		h.fail(w, errBadRequest)
		return nil, false
	}
	f, _, err := r.FormFile(field)
	if err != nil {
		h.fail(w, &domain.ValidationError{Fields: map[string]string{field: "required"}})
		return nil, false
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxUpload))
	if err != nil {
		h.fail(w, errBadRequest)
		return nil, false
	}
	return data, true
}

func (h handlerCtx) uploadPicture(w http.ResponseWriter, r *http.Request) {
	data, ok := h.readUpload(w, r, "picture")
	if !ok {
		return
	}
	if _, err := h.api.Catalog.SetPicture(r.Context(), data, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.adminSite(w, r)
}

func (h handlerCtx) deletePicture(w http.ResponseWriter, r *http.Request) {
	if err := h.api.Catalog.RemovePicture(r.Context(), h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.adminSite(w, r)
}

func (h handlerCtx) audit(w http.ResponseWriter, r *http.Request) {
	f := domain.AuditFilter{}
	f.Limit, f.Offset = page(r)
	if raw := r.URL.Query().Get("userId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			h.fail(w, &domain.ValidationError{Fields: map[string]string{"userId": "invalid"}})
			return
		}
		f.TargetUserID = &id
	}
	entries, total, err := h.api.Users.AuditLog(r.Context(), f)
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": toAudit(r.Context(), n, entries), "total": total})
}
