package httpapi

import (
	"net/http"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// registerModules adds the admin routes of the module registry and of the
// feed module.
func registerModules(h handlerCtx, routes map[string]http.HandlerFunc) {
	routes["GET /api/v1/admin/modules"] = h.listModules
	routes["PUT /api/v1/admin/modules/{code}"] = h.setModule
	registerFeedAdmin(h, routes)
}

type moduleNavDTO struct {
	Key      string `json:"key"`
	Path     string `json:"path"`
	LabelKey string `json:"labelKey"`
	Icon     string `json:"icon"`
}

type moduleDTO struct {
	Code           string     `json:"code"`
	NameKey        string     `json:"nameKey"`
	DescriptionKey string     `json:"descriptionKey"`
	Enabled        bool       `json:"enabled"`
	DefaultEnabled bool       `json:"defaultEnabled"`
	UpdatedAt      *time.Time `json:"updatedAt"`
	UpdatedBy      *userRef   `json:"updatedBy"`
}

func (h handlerCtx) toModule(r *http.Request, n *namer, v domain.ModuleView) moduleDTO {
	d := moduleDTO{Code: v.Code, NameKey: v.NameKey, DescriptionKey: v.DescriptionKey, Enabled: v.Enabled,
		DefaultEnabled: v.DefaultEnabled, UpdatedAt: v.UpdatedAt}
	if v.UpdatedBy != nil {
		d.UpdatedBy = &userRef{ID: *v.UpdatedBy, Name: n.userName(r.Context(), *v.UpdatedBy)}
	}
	return d
}

func (h handlerCtx) listModules(w http.ResponseWriter, r *http.Request) {
	list, err := h.api.Modules.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]moduleDTO, 0, len(list))
	for _, v := range list {
		out = append(out, h.toModule(r, n, v))
	}
	WriteJSON(w, http.StatusOK, out)
}

func (h handlerCtx) setModule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decode(r, &in); err != nil || in.Enabled == nil {
		h.fail(w, errBadRequest)
		return
	}
	v, err := h.api.Modules.SetEnabled(r.Context(), r.PathValue("code"), *in.Enabled, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, h.toModule(r, n, v))
}

// myModules lists the modules that are on with their cabinet navigation:
// the cabinet builds its menu from it.
func (h handlerCtx) myModules(w http.ResponseWriter, r *http.Request) {
	list, err := h.api.Modules.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	type mine struct {
		Code string         `json:"code"`
		Nav  []moduleNavDTO `json:"nav"`
	}
	out := []mine{}
	for _, v := range list {
		if !v.Enabled {
			continue
		}
		m := mine{Code: v.Code, Nav: []moduleNavDTO{}}
		for _, e := range v.Nav {
			m.Nav = append(m.Nav, moduleNavDTO(e))
		}
		out = append(out, m)
	}
	WriteJSON(w, http.StatusOK, out)
}
