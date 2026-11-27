package httpapi

import (
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
	Public      bool                `json:"public"`
	ValueType   domain.DataType     `json:"valueType,omitempty"`
	Value       json.RawMessage     `json:"value,omitempty"`
	Columns     []domain.DataColumn `json:"columns,omitempty"`
	RowCount    int                 `json:"rowCount"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	UpdatedBy   *userRef            `json:"updatedBy"`
}

type dataRowDTO struct {
	ID       int64                      `json:"id,omitempty"`
	Position int                        `json:"position"`
	Cells    map[string]json.RawMessage `json:"cells"`
}

func (h handlerCtx) toDataItem(r *http.Request, n *namer, it domain.DataItem) dataItemDTO {
	d := dataItemDTO{Key: it.Key, Kind: it.Kind, Description: it.Description, Public: it.Public, ValueType: it.ValueType,
		Value: it.Value, Columns: it.Columns, RowCount: it.RowCount, UpdatedAt: it.UpdatedAt}
	if it.Kind == domain.DataTable && d.Columns == nil {
		d.Columns = []domain.DataColumn{}
	}
	if it.UpdatedBy != nil && n != nil {
		d.UpdatedBy = &userRef{ID: *it.UpdatedBy, Name: n.userName(r.Context(), *it.UpdatedBy)}
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
	Public      *bool                `json:"public"`
	ValueType   *domain.DataType     `json:"valueType"`
	Value       json.RawMessage      `json:"value"`
	Columns     *[]domain.DataColumn `json:"columns"`
}

func (in dataIn) input() domain.DataInput {
	return domain.DataInput{Key: in.Key, Kind: in.Kind, Description: in.Description, Public: in.Public,
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
