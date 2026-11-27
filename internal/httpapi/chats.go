package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// registerChats adds the admin routes for chats, labels and notifications.
func registerChats(h handlerCtx, routes map[string]http.HandlerFunc) {
	routes["GET /api/v1/admin/chats"] = h.listChats
	routes["POST /api/v1/admin/chats/{id}/accept"] = h.chatAction(h.api.Chats.Accept)
	routes["POST /api/v1/admin/chats/{id}/leave"] = h.chatAction(h.api.Chats.Leave)
	routes["PUT /api/v1/admin/chats/{id}/labels"] = h.setLabels(domain.TargetChat)
	routes["PUT /api/v1/admin/topics/{id}/labels"] = h.setLabels(domain.TargetTopic)
	routes["PATCH /api/v1/admin/topics/{id}"] = h.renameTopic
	routes["POST /api/v1/admin/chats/{id}/test"] = h.testSend(h.api.Notifier.TestChat)
	routes["POST /api/v1/admin/topics/{id}/test"] = h.testSend(h.api.Notifier.TestTopic)
	routes["POST /api/v1/admin/labels/{id}/test"] = h.testSend(h.api.Notifier.TestLabel)
	routes["GET /api/v1/admin/outbox/{id}"] = h.outboxStatus
	routes["GET /api/v1/admin/labels"] = h.listLabels
	routes["POST /api/v1/admin/labels"] = h.createLabel
	routes["PATCH /api/v1/admin/labels/{id}"] = h.updateLabel
	routes["DELETE /api/v1/admin/labels/{id}"] = h.deleteLabel
	routes["GET /api/v1/admin/settings/notifications"] = h.getNotifications
	routes["PUT /api/v1/admin/settings/notifications"] = h.putNotifications
}

type addedByDTO struct {
	ExternalID string `json:"externalId"`
	UserID     *int64 `json:"userId"`
}

type topicDTO struct {
	ID              int64     `json:"id"`
	ExternalID      string    `json:"externalId"`
	Title           string    `json:"title"`
	TitleSetByAdmin bool      `json:"titleSetByAdmin"`
	Gone            bool      `json:"gone"`
	LastSeenAt      time.Time `json:"lastSeenAt"`
	LabelIDs        []int64   `json:"labelIds"`
}

type chatDTO struct {
	ID          int64      `json:"id"`
	Messenger   string     `json:"messenger"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	IsForum     bool       `json:"isForum"`
	State       string     `json:"state"`
	BotStatus   string     `json:"botStatus"`
	CanSend     bool       `json:"canSend"`
	AddedBy     addedByDTO `json:"addedBy"`
	LeaveAt     *time.Time `json:"leaveAt"`
	LeftAt      *time.Time `json:"leftAt"`
	FirstSeenAt time.Time  `json:"firstSeenAt"`
	LastSeenAt  time.Time  `json:"lastSeenAt"`
	LabelIDs    []int64    `json:"labelIds"`
	Topics      []topicDTO `json:"topics"`
}

func ids(v []int64) []int64 {
	if v == nil {
		return []int64{}
	}
	return v
}

func toChat(c domain.ChatView) chatDTO {
	out := chatDTO{ID: c.ID, Messenger: c.Messenger, Kind: c.Kind, Title: c.Title, IsForum: c.IsForum, State: string(c.State),
		BotStatus: c.BotStatus, CanSend: c.CanSend, AddedBy: addedByDTO{c.AddedByExternalID, c.AddedByUserID},
		LeaveAt: c.LeaveAt, LeftAt: c.LeftAt, FirstSeenAt: c.FirstSeenAt, LastSeenAt: c.LastSeenAt,
		LabelIDs: ids(c.LabelIDs), Topics: []topicDTO{}}
	for _, t := range c.Topics {
		out.Topics = append(out.Topics, topicDTO{ID: t.ID, ExternalID: t.ExternalID, Title: t.Title, TitleSetByAdmin: t.TitleSetByAdmin,
			Gone: t.Gone, LastSeenAt: t.LastSeenAt, LabelIDs: ids(t.LabelIDs)})
	}
	return out
}

func (h handlerCtx) listChats(w http.ResponseWriter, r *http.Request) {
	list, err := h.api.Chats.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]chatDTO, 0, len(list))
	for _, c := range list {
		out = append(out, toChat(c))
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h handlerCtx) chatAction(fn func(ctx context.Context, id int64, a domain.Actor) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r, "id")
		if err == nil {
			err = fn(r.Context(), id, h.actor(r))
		}
		if err != nil {
			h.fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h handlerCtx) setLabels(targetType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		if err := h.api.Labels.SetLabels(r.Context(), domain.LabelTarget{Type: targetType, ID: id}, in.LabelIDs, h.actor(r)); err != nil {
			h.fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h handlerCtx) renameTopic(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	if err := h.api.Chats.RenameTopic(r.Context(), id, in.Title, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handlerCtx) testSend(fn func(ctx context.Context, id int64) ([]int64, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r, "id")
		if err != nil {
			h.fail(w, err)
			return
		}
		got, err := fn(r.Context(), id)
		if err != nil {
			h.fail(w, err)
			return
		}
		WriteJSON(w, http.StatusAccepted, map[string]any{"outboxIds": ids(got)})
	}
}

func (h handlerCtx) outboxStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	it, err := h.api.Notifier.Status(r.Context(), id)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"state": string(it.State), "lastError": it.LastError})
}

type labelTargetDTO struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
}

type labelDTO struct {
	ID                 int64            `json:"id"`
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	Targets            []labelTargetDTO `json:"targets"`
	UsedByApplications bool             `json:"usedByApplications"`
}

func toLabel(v domain.LabelView) labelDTO {
	out := labelDTO{ID: v.ID, Name: v.Name, Description: v.Description, Targets: []labelTargetDTO{}, UsedByApplications: v.UsedByApplications}
	for _, t := range v.Targets {
		out.Targets = append(out.Targets, labelTargetDTO(t))
	}
	return out
}

func (h handlerCtx) listLabels(w http.ResponseWriter, r *http.Request) {
	list, err := h.api.Labels.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]labelDTO, 0, len(list))
	for _, v := range list {
		out = append(out, toLabel(v))
	}
	WriteJSON(w, http.StatusOK, out)
}

type labelIn struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h handlerCtx) createLabel(w http.ResponseWriter, r *http.Request) {
	var in labelIn
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	l, err := h.api.Labels.Create(r.Context(), in.Name, in.Description, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, toLabel(domain.LabelView{Label: l}))
}

func (h handlerCtx) updateLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	var in labelIn
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	l, err := h.api.Labels.Update(r.Context(), id, in.Name, in.Description, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, toLabel(domain.LabelView{Label: l}))
}

func (h handlerCtx) deleteLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err == nil {
		err = h.api.Labels.Delete(r.Context(), id, h.actor(r))
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type notificationsDTO struct {
	ApplicationsLabelID *int64 `json:"applicationsLabelId"`
}

func (h handlerCtx) getNotifications(w http.ResponseWriter, r *http.Request) {
	n, err := h.api.Labels.Notifications(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, notificationsDTO(n))
}

func (h handlerCtx) putNotifications(w http.ResponseWriter, r *http.Request) {
	var in notificationsDTO
	if err := decode(r, &in); err != nil {
		h.fail(w, err)
		return
	}
	if err := h.api.Labels.SaveNotifications(r.Context(), domain.NotificationSettings(in), h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	h.getNotifications(w, r)
}
