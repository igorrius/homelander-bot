package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// maxDataBody caps data request bodies: a 64 KiB JSON value or up to 1000 rows.
const maxDataBody = 2 << 20

func registerData(h handlerCtx, routes map[string]http.HandlerFunc) {
	routes["GET /api/v1/admin/assets/{id}"] = h.adminAsset
	routes["POST /api/v1/admin/data/assets"] = h.uploadDataImage
	routes["GET /api/v1/admin/data"] = h.listData
	routes["POST /api/v1/admin/data"] = h.createData
	routes["GET /api/v1/admin/data/{key}"] = h.getData
	routes["PATCH /api/v1/admin/data/{key}"] = h.patchData
	routes["DELETE /api/v1/admin/data/{key}"] = h.deleteData
	routes["PUT /api/v1/admin/data/{key}/rows"] = h.putDataRows
	routes["DELETE /api/v1/admin/data/{key}/rows/{rowId}"] = h.deleteDataRow
}

type userRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type dataItemDTO struct {
	Key         string              `json:"key"`
	Kind        domain.DataKind     `json:"kind"`
	Description string              `json:"description"`
	Visibility  domain.Visibility   `json:"visibility"`
	ValueType   domain.DataType     `json:"valueType,omitempty"`
	Value       json.RawMessage     `json:"value,omitempty"`
	Columns     []domain.DataColumn `json:"columns,omitempty"`
	RowCount    int                 `json:"rowCount"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	UpdatedBy   *userRef            `json:"updatedBy"`
	// Owner is the code of the module that writes the item, or null.
	Owner        *string `json:"owner"`
	OwnerEnabled bool    `json:"ownerEnabled"`
}

type dataRowDTO struct {
	ID       int64                      `json:"id,omitempty"`
	Position int                        `json:"position"`
	Cells    map[string]json.RawMessage `json:"cells"`
}

func (h handlerCtx) toDataItem(r *http.Request, n *namer, it domain.DataItem) dataItemDTO {
	d := dataItemDTO{Key: it.Key, Kind: it.Kind, Description: it.Description, Visibility: it.Visibility, ValueType: it.ValueType,
		Value: it.Value, Columns: it.Columns, RowCount: it.RowCount, UpdatedAt: it.UpdatedAt}
	if it.Kind == domain.DataTable && d.Columns == nil {
		d.Columns = []domain.DataColumn{}
	}
	if it.UpdatedBy != nil && n != nil {
		d.UpdatedBy = &userRef{ID: *it.UpdatedBy, Name: n.userName(r.Context(), *it.UpdatedBy)}
	}
	if code := domain.ModuleOf(it.Owner); code != "" {
		d.Owner = &code
		if h.api.Modules != nil {
			d.OwnerEnabled, _ = h.api.Modules.Enabled(r.Context(), code)
		}
	}
	return d
}

func toDataRows(rows []domain.DataRow) []dataRowDTO {
	out := make([]dataRowDTO, 0, len(rows))
	for _, r := range rows {
		cells := r.Cells
		if cells == nil {
			cells = map[string]json.RawMessage{}
		}
		out = append(out, dataRowDTO{ID: r.ID, Position: r.Position, Cells: cells})
	}
	return out
}

func (h handlerCtx) writeDataItem(w http.ResponseWriter, r *http.Request, status int, it domain.DataItem) {
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, status, h.toDataItem(r, n, it))
}

func (h handlerCtx) listData(w http.ResponseWriter, r *http.Request) {
	items, err := h.api.Data.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make([]dataItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, h.toDataItem(r, n, it))
	}
	WriteJSON(w, http.StatusOK, out)
}

type dataIn struct {
	Key         string               `json:"key"`
	Kind        domain.DataKind      `json:"kind"`
	Description *string              `json:"description"`
	Visibility  *domain.Visibility   `json:"visibility"`
	ValueType   *domain.DataType     `json:"valueType"`
	Value       json.RawMessage      `json:"value"`
	Columns     *[]domain.DataColumn `json:"columns"`
}

func (in dataIn) input() domain.DataInput {
	return domain.DataInput{Key: in.Key, Kind: in.Kind, Description: in.Description, Visibility: in.Visibility,
		ValueType: in.ValueType, Value: in.Value, Columns: in.Columns}
}

func (h handlerCtx) createData(w http.ResponseWriter, r *http.Request) {
	var in dataIn
	if err := decodeLimit(r, &in, maxDataBody); err != nil {
		h.fail(w, err)
		return
	}
	it, err := h.api.Data.Create(r.Context(), in.input(), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writeDataItem(w, r, http.StatusCreated, it)
}

func (h handlerCtx) getData(w http.ResponseWriter, r *http.Request) {
	v, err := h.api.Data.Get(r.Context(), r.PathValue("key"))
	if err != nil {
		h.fail(w, err)
		return
	}
	n, err := newNamer(r.Context(), h.api)
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"item": h.toDataItem(r, n, v.Item), "rows": toDataRows(v.Rows)})
}

func (h handlerCtx) patchData(w http.ResponseWriter, r *http.Request) {
	var in dataIn
	if err := decodeLimit(r, &in, maxDataBody); err != nil || in.Key != "" || in.Kind != "" {
		h.fail(w, errBadRequest)
		return
	}
	it, err := h.api.Data.Patch(r.Context(), r.PathValue("key"), in.input(), h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	h.writeDataItem(w, r, http.StatusOK, it)
}

func (h handlerCtx) deleteData(w http.ResponseWriter, r *http.Request) {
	if err := h.api.Data.Delete(r.Context(), r.PathValue("key"), h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h handlerCtx) putDataRows(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Rows []dataRowDTO `json:"rows"`
	}
	if err := decodeLimit(r, &in, maxDataBody); err != nil {
		h.fail(w, err)
		return
	}
	rows := make([]domain.DataRow, 0, len(in.Rows))
	for _, d := range in.Rows {
		rows = append(rows, domain.DataRow{ID: d.ID, Position: d.Position, Cells: d.Cells})
	}
	out, err := h.api.Data.UpsertRows(r.Context(), r.PathValue("key"), rows, h.actor(r))
	if err != nil {
		h.fail(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"rows": toDataRows(out)})
}

func (h handlerCtx) deleteDataRow(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "rowId")
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.api.Data.DeleteRows(r.Context(), r.PathValue("key"), []int64{id}, h.actor(r)); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// publicItem is what a site page may read about an item: no description, no
// author, images as URLs made by image.
func publicItem(v domain.DataView, image func(id int64) string) map[string]any {
	it := v.Item
	if it.Kind == domain.DataValue {
		return map[string]any{"kind": "value", "type": it.ValueType, "value": publicCell(it.ValueType, it.Value, image)}
	}
	types := map[string]domain.DataType{}
	for _, c := range it.Columns {
		types[c.Name] = c.Type
	}
	rows := make([]map[string]any, 0, len(v.Rows))
	for _, r := range v.Rows {
		row := map[string]any{"id": r.ID}
		for name, raw := range r.Cells {
			row[name] = publicCell(types[name], raw, image)
		}
		rows = append(rows, row)
	}
	cols := it.Columns
	if cols == nil {
		cols = []domain.DataColumn{}
	}
	return map[string]any{"kind": "table", "columns": cols, "rows": rows}
}

// publicCell turns an image id into its URL and leaves other values as they are.
func publicCell(t domain.DataType, raw json.RawMessage, image func(id int64) string) any {
	if t == domain.DataImage {
		if id, err := strconv.ParseInt(string(raw), 10, 64); err == nil {
			return image(id)
		}
	}
	if len(raw) == 0 {
		return nil
	}
	return raw
}

// writePublic writes a public data response with CORS, short caching and an ETag.
func writePublic(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal")
		return
	}
	sum := sha256.Sum256(body)
	tag := `"` + hex.EncodeToString(sum[:])[:32] + `"`
	h := w.Header()
	h.Set("ETag", tag)
	h.Set("Cache-Control", "public, max-age=30")
	if r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h handlerCtx) publicData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	list, err := h.api.Data.Public(r.Context(), ClientIP(r, h.api.Cfg.TrustProxy))
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make(map[string]any, len(list))
	for _, v := range list {
		out[v.Item.Key] = publicItem(v, publicAssetURL)
	}
	writePublic(w, r, out)
}

func (h handlerCtx) publicDataOne(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	v, err := h.api.Data.PublicOne(r.Context(), r.PathValue("key"), ClientIP(r, h.api.Cfg.TrustProxy))
	if err != nil {
		h.fail(w, err)
		return
	}
	writePublic(w, r, publicItem(v, publicAssetURL))
}

// publicAssetURL is the URL of an asset a public item refers to.
func publicAssetURL(id int64) string { return "/api/v1/assets/" + strconv.FormatInt(id, 10) }

// myData returns what the signed-in user may read on the site: public items,
// and residents items for a resident or an admin, in the shape of
// /public/data. Images of residents items are signed, expiring URLs, because
// the page's frame sends no cookie. Only our shell calls it: no CORS.
func (h handlerCtx) myData(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r)
	list, err := h.api.Data.ForUser(r.Context(), u)
	if err != nil {
		h.fail(w, err)
		return
	}
	out := make(map[string]any, len(list))
	for _, v := range list {
		image := publicAssetURL
		if v.Item.Visibility != domain.VisPublic {
			image = h.api.Data.SignedAssetURL
		}
		out[v.Item.Key] = publicItem(v, image)
	}
	body, err := json.Marshal(out)
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(body)
}
