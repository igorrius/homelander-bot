package httpapi

import (
	"net/http"
	"strconv"
)

func registerData(h handlerCtx, routes map[string]http.HandlerFunc) {
	routes["GET /api/v1/admin/assets/{id}"] = h.adminAsset
	routes["POST /api/v1/admin/data/assets"] = h.uploadDataImage
}

func (h handlerCtx) adminAsset(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, err)
		return
	}
	h.serveAsset(w, r, id, "private, no-store")
}

func adminAssetURL(id int64) string { return "/api/v1/admin/assets/" + strconv.FormatInt(id, 10) }

func (h handlerCtx) uploadDataImage(w http.ResponseWriter, r *http.Request) {
	data, ok := h.readUpload(w, r, "file")
	if !ok {
		return
	}
	a, err := h.api.Catalog.UploadImage(r.Context(), data, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"id": a.ID, "url": adminAssetURL(a.ID)})
}
