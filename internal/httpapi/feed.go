package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

func registerFeedAdmin(h handlerCtx, routes map[string]http.HandlerFunc) {
	routes["GET /api/v1/admin/feed/settings"] = h.feedSettings
	routes["PUT /api/v1/admin/feed/settings"] = h.saveFeedSettings
	routes["GET /api/v1/admin/feed/posts"] = h.listPosts
	routes["POST /api/v1/admin/feed/posts"] = h.createPost
	routes["GET /api/v1/admin/feed/posts/{id}"] = h.getPost
	routes["PATCH /api/v1/admin/feed/posts/{id}"] = h.patchPost
	routes["POST /api/v1/admin/feed/posts/{id}/publish"] = h.publishPost
	routes["DELETE /api/v1/admin/feed/posts/{id}"] = h.deletePost
}

// registerFeedCabinet adds the cabinet's feed routes (any signed-in user).
func registerFeedCabinet(h handlerCtx) {
	m := h.s.API()
	m.Handle("GET /api/v1/modules", RequireUser(http.HandlerFunc(h.myModules)))
	m.Handle("GET /api/v1/feed/posts", RequireUser(http.HandlerFunc(h.cabinetPosts)))
	m.Handle("GET /api/v1/feed/posts/{id}", RequireUser(http.HandlerFunc(h.cabinetPost)))
	m.Handle("GET /api/v1/feed/posts/{id}/image", RequireUser(http.HandlerFunc(h.cabinetPostImage)))
}

func (h handlerCtx) feedSettings(w http.ResponseWriter, r *http.Request) {
	if err := h.api.Modules.Require(r.Context(), domain.FeedCode); err != nil {
		h.fail(w, err)
		return
	}
	st, err := h.api.Feed.Settings(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"defaultLabelIds": ids(st.DefaultLabelIDs)})
}

func (h handlerCtx) saveFeedSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DefaultLabelIDs []int64 `json:"defaultLabelIds"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	st, err := h.api.Feed.SaveSettings(r.Context(), domain.FeedSettings{DefaultLabelIDs: in.DefaultLabelIDs}, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"defaultLabelIds": ids(st.DefaultLabelIDs)})
}

type deliveryCountDTO struct {
	Pending int `json:"pending"`
	Sent    int `json:"sent"`
	Failed  int `json:"failed"`
}

type deliveryDTO struct {
	ID         int64      `json:"id"`
	ChatTitle  string     `json:"chatTitle"`
	TopicTitle string     `json:"topicTitle"`
	LabelID    *int64     `json:"labelId"`
	State      string     `json:"state"`
	Error      string     `json:"error"`
	SentAt     *time.Time `json:"sentAt"`
}

type adminPostDTO struct {
	ID              int64            `json:"id"`
	Body            string           `json:"body"`
	ImageAssetID    *int64           `json:"imageAssetId"`
	ImageURL        *string          `json:"imageUrl"`
	State           domain.PostState `json:"state"`
	PublishedAt     *time.Time       `json:"publishedAt"`
	LabelIDs        []int64          `json:"labelIds"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	CreatedBy       *userRef         `json:"createdBy"`
	UpdatedBy       *userRef         `json:"updatedBy"`
	Delivery        deliveryCountDTO `json:"delivery"`
	DeliveryExpired bool             `json:"deliveryExpired"`
	Deliveries      []deliveryDTO    `json:"deliveries,omitempty"`
}

// placeNames names outbox destinations: chat titles by messenger and
// external id, topic titles by chat and thread.
type placeNames struct {
	chats  map[string]string
	topics map[string]string
}

func (h handlerCtx) placeNames(ctx context.Context) placeNames {
	p := placeNames{chats: map[string]string{}, topics: map[string]string{}}
	if cs, err := h.api.Chats.List(ctx); err == nil {
		for _, c := range cs {
			key := c.Messenger + "/" + c.ExternalID
			p.chats[key] = c.Title
			for _, t := range c.Topics {
				p.topics[key+"/"+t.ExternalID] = t.Title
			}
		}
	}
	return p
}

// toAdminPost fills the delivery counts from rows; withRows also lists them.
func (h handlerCtx) toAdminPost(r *http.Request, n *namer, p domain.FeedPost, rows []domain.OutboxItem, withRows bool) adminPostDTO {
	d := adminPostDTO{ID: p.ID, Body: p.Body, ImageAssetID: p.ImageAssetID, State: p.State, PublishedAt: p.PublishedAt,
		LabelIDs: ids(p.LabelIDs), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
	if p.ImageAssetID != nil {
		u := adminAssetURL(*p.ImageAssetID)
		d.ImageURL = &u
	}
	ref := func(id *int64) *userRef {
		if id == nil {
			return nil
		}
		return &userRef{ID: *id, Name: n.userName(r.Context(), *id)}
	}
	d.CreatedBy, d.UpdatedBy = ref(p.CreatedBy), ref(p.UpdatedBy)
	for _, it := range rows {
		switch it.State {
		case domain.OutboxPending:
			d.Delivery.Pending++
		case domain.OutboxSent:
			d.Delivery.Sent++
		case domain.OutboxFailed:
			d.Delivery.Failed++
		}
	}
	d.DeliveryExpired = p.State == domain.PostPublished && len(p.LabelIDs) > 0 && len(rows) == 0 && !h.api.Feed.DeliveryKept(p)
	if withRows {
		names := h.placeNames(r.Context())
		d.Deliveries = []deliveryDTO{}
		for _, it := range rows {
			key := it.Messenger + "/" + it.To.ChatID
			dd := deliveryDTO{ID: it.ID, ChatTitle: names.chats[key], LabelID: it.LabelID, State: string(it.State),
				Error: it.LastError, SentAt: it.SentAt}
			if it.To.ThreadID != "" {
				dd.TopicTitle = names.topics[key+"/"+it.To.ThreadID]
			}
			d.Deliveries = append(d.Deliveries, dd)
		}
	}
	return d
}

func (h handlerCtx) writePost(w http.ResponseWriter, r *http.Request, status int, p domain.FeedPost) {
	rows, err := h.api.Feed.Deliveries(r.Context(), p.ID)
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, status, h.toAdminPost(r, n, p, rows, true))
}

func (h handlerCtx) listPosts(w http.ResponseWriter, r *http.Request) {
	limit, offset := page(r)
	posts, total, err := h.api.Feed.List(r.Context(), limit, offset)
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]adminPostDTO, 0, len(posts))
	for _, p := range posts {
		var rows []domain.OutboxItem
		if p.State == domain.PostPublished {
			if rows, err = h.api.Feed.Deliveries(r.Context(), p.ID); err != nil {
				h.fail(w, err)
				return
			}
		}
		items = append(items, h.toAdminPost(r, n, p, rows, false))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

type postIn struct {
	Body         string   `json:"body"`
	ImageAssetID *int64   `json:"imageAssetId"`
	LabelIDs     *[]int64 `json:"labelIds"`
}

func (in postIn) input() domain.PostInput {
	return domain.PostInput{Body: in.Body, ImageAssetID: in.ImageAssetID, LabelIDs: in.LabelIDs}
}

func (h handlerCtx) createPost(w http.ResponseWriter, r *http.Request) {
	var in postIn
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	p, err := h.api.Feed.Create(r.Context(), in.input(), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writePost(w, r, http.StatusCreated, p)
}

func (h handlerCtx) getPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	p, err := h.api.Feed.Get(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writePost(w, r, http.StatusOK, p)
}

func (h handlerCtx) patchPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in postIn
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	p, err := h.api.Feed.Update(r.Context(), id, in.input(), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writePost(w, r, http.StatusOK, p)
}

func (h handlerCtx) publishPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in struct {
		LabelIDs []int64 `json:"labelIds"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	p, err := h.api.Feed.Publish(r.Context(), id, in.LabelIDs, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writePost(w, r, http.StatusOK, p)
}

func (h handlerCtx) deletePost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err == nil {
		err = h.api.Feed.Delete(r.Context(), id, h.actor(r))
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type cabinetPostDTO struct {
	ID          int64     `json:"id"`
	Body        string    `json:"body"`
	ImageURL    *string   `json:"imageUrl"`
	PublishedAt time.Time `json:"publishedAt"`
	AuthorName  string    `json:"authorName"`
}

// authors caches the first names of post authors for one request.
type authors struct {
	h     handlerCtx
	names map[int64]string
}

func (a authors) name(ctx context.Context, id *int64) string {
	if id == nil {
		return ""
	}
	if n, ok := a.names[*id]; ok {
		return n
	}
	n := ""
	if u, err := a.h.api.Users.Get(ctx, *id); err == nil {
		n = u.FirstName
	}
	a.names[*id] = n
	return n
}

func (a authors) post(ctx context.Context, p domain.FeedPost) cabinetPostDTO {
	d := cabinetPostDTO{ID: p.ID, Body: p.Body, AuthorName: a.name(ctx, p.CreatedBy)}
	if p.PublishedAt != nil {
		d.PublishedAt = *p.PublishedAt
	}
	if p.ImageAssetID != nil {
		u := "/api/v1/feed/posts/" + strconv.FormatInt(p.ID, 10) + "/image"
		d.ImageURL = &u
	}
	return d
}

func (h handlerCtx) cabinetPosts(w http.ResponseWriter, r *http.Request) {
	posts, next, err := h.api.Feed.Published(r.Context(), r.URL.Query().Get("cursor"), domain.FeedPageSize)
	if err != nil {
		h.fail(w, err)
		return
	}
	a := authors{h, map[int64]string{}}
	items := make([]cabinetPostDTO, 0, len(posts))
	for _, p := range posts {
		items = append(items, a.post(r.Context(), p))
	}
	var nextCursor *string
	if next != "" {
		nextCursor = &next
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": nextCursor})
}

func (h handlerCtx) cabinetPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	p, err := h.api.Feed.PublishedOne(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, authors{h, map[int64]string{}}.post(r.Context(), p))
}

// cabinetPostImage serves a published post's image to a signed-in user; older
// posts are not in feed.posts, so their images are served from here.
func (h handlerCtx) cabinetPostImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	p, err := h.api.Feed.PublishedOne(r.Context(), id)
	if err == nil && p.ImageAssetID == nil {
		err = domain.ErrNotFound
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	h.serveAsset(w, r, *p.ImageAssetID, "private, max-age=3600")
}
