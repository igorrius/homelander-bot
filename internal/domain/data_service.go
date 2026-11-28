package domain

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/igorrius/homelander-bot/internal/ratelimit"
)

// DataService manages the data space: values and tables admins (later agents
// and modules) keep, and the public and residents reads of them.
type DataService struct {
	R       Repos
	Now     Clock
	Limits  *ratelimit.Limiter
	Locales []string
	// Secret signs the expiring asset URLs of residents-level images.
	Secret []byte
	// Modules reserves module key prefixes and hides the data of modules
	// that are off; nil when there are no modules.
	Modules *ModuleService
}

// DataInput creates an item or, in Patch, changes it; nil fields stay as they
// are. A new item without Visibility is admins-only.
type DataInput struct {
	Key         string
	Kind        DataKind
	Description *string
	Visibility  *Visibility
	ValueType   *DataType
	Value       json.RawMessage
	Columns     *[]DataColumn
}

// DataView is an item with its rows (tables only).
type DataView struct {
	Item DataItem
	Rows []DataRow
}

const (
	maxDataKey     = 100
	maxDataDesc    = 1000
	maxDataText    = 2000
	maxDataJSON    = 64 << 10
	maxDataRows    = 1000
	maxDataColumns = 50
	publicLimit    = 120
	publicWindow   = time.Minute
)

var (
	dataKeyRe  = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	dataColRe  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	dataPhone  = regexp.MustCompile(`^[0-9+() -]{3,30}$`)
	valueTypes = []DataType{DataText, DataLocalizedText, DataNumber, DataBool, DataImage, DataURL, DataPhone, DataJSON}
	colTypes   = []DataType{DataText, DataLocalizedText, DataNumber, DataBool, DataImage, DataURL, DataPhone}
)

func (s *DataService) audit(ctx context.Context, a Actor, action, key string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: DataTarget(key), Details: a.AuditDetails(details), IP: a.IP})
}

// List returns every item without rows.
func (s *DataService) List(ctx context.Context) ([]DataItem, error) { return s.R.Data.List(ctx) }

// Get returns an item with its rows.
func (s *DataService) Get(ctx context.Context, key string) (DataView, error) {
	it, err := s.R.Data.GetByKey(ctx, key)
	if err != nil {
		return DataView{}, err
	}
	return s.view(ctx, it)
}

func (s *DataService) view(ctx context.Context, it DataItem) (DataView, error) {
	v := DataView{Item: it}
	if it.Kind == DataTable {
		rows, err := s.R.Data.Rows(ctx, it.ID)
		if err != nil {
			return DataView{}, err
		}
		v.Rows = rows
	}
	return v, nil
}

// Create adds an item.
func (s *DataService) Create(ctx context.Context, in DataInput, a Actor) (DataItem, error) {
	v := validation{}
	if len(in.Key) > maxDataKey || !dataKeyRe.MatchString(in.Key) {
		v.add("key", "invalid")
	} else if s.Modules.Reserved(in.Key) {
		v.add("key", "key_reserved")
	}
	now := s.Now()
	it := DataItem{Key: in.Key, Kind: in.Kind, Visibility: VisAdmins, CreatedBy: a.UserID, UpdatedBy: a.UserID, CreatedAt: now, UpdatedAt: now}
	if in.Description != nil {
		it.Description = strings.TrimSpace(*in.Description)
	}
	if in.Visibility != nil {
		if !in.Visibility.Valid() {
			v.add("visibility", "invalid")
		}
		it.Visibility = *in.Visibility
	}
	switch in.Kind {
	case DataValue:
		if in.Columns != nil {
			v.add("columns", "invalid")
		}
		if in.ValueType == nil {
			v.add("valueType", "required")
		} else {
			it.ValueType = *in.ValueType
		}
		it.Value = in.Value
	case DataTable:
		if in.ValueType != nil || in.Value != nil {
			v.add("valueType", "invalid")
		}
		it.Columns = []DataColumn{}
		if in.Columns != nil {
			it.Columns = *in.Columns
		}
	default:
		v.add("kind", "invalid")
	}
	if err := v.err(); err != nil {
		return DataItem{}, err
	}
	if err := s.check(ctx, &it); err != nil {
		return DataItem{}, err
	}
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.R.Data.Create(ctx, &it); err != nil {
			if errors.Is(err, ErrConflict) {
				return &ValidationError{Fields: map[string]string{"key": "taken"}}
			}
			return err
		}
		if err := s.R.Data.SetAssetRefs(ctx, it.ID, imageRefs(it, nil)); err != nil {
			return err
		}
		return s.audit(ctx, a, ActDataCreated, it.Key, map[string]any{"kind": string(it.Kind), "visibility": string(it.Visibility)})
	})
	return it, err
}

// check validates the description, the value or the columns of it, and
// normalises them.
func (s *DataService) check(ctx context.Context, it *DataItem) error {
	v := validation{}
	if utf8.RuneCountInString(it.Description) > maxDataDesc {
		v.add("description", "too_long")
	}
	if it.Kind == DataValue {
		if !slices.Contains(valueTypes, it.ValueType) {
			v.add("valueType", "invalid")
		} else {
			if len(it.Value) == 0 {
				it.Value = json.RawMessage("null")
			}
			norm, reason := s.checkValue(ctx, it.ValueType, it.Value)
			if reason != "" {
				v.add("value", reason)
			}
			it.Value = norm
		}
	} else if !validColumns(it.Columns) {
		v.add("columns", "invalid")
	}
	return v.err()
}

func validColumns(cols []DataColumn) bool {
	if len(cols) > maxDataColumns {
		return false
	}
	seen := map[string]bool{}
	for i, c := range cols {
		if !dataColRe.MatchString(c.Name) || len(c.Name) > 50 || seen[c.Name] || !slices.Contains(colTypes, c.Type) ||
			utf8.RuneCountInString(c.Label) > 100 {
			return false
		}
		seen[c.Name] = true
		cols[i].Label = strings.TrimSpace(c.Label)
	}
	return true
}

// checkValue returns the compacted value, or a reason it is not acceptable.
func (s *DataService) checkValue(ctx context.Context, t DataType, raw json.RawMessage) (json.RawMessage, string) {
	if len(raw) > maxDataJSON {
		return raw, "too_long"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw, "invalid"
	}
	norm := json.RawMessage(buf.Bytes())
	if string(norm) == "null" {
		return norm, ""
	}
	switch t {
	case DataText:
		var str string
		if json.Unmarshal(norm, &str) != nil {
			return norm, "invalid"
		}
		if utf8.RuneCountInString(str) > maxDataText {
			return norm, "too_long"
		}
	case DataLocalizedText:
		var m map[string]string
		if json.Unmarshal(norm, &m) != nil {
			return norm, "invalid"
		}
		for loc, str := range m {
			if !slices.Contains(s.Locales, loc) {
				return norm, "invalid"
			}
			if utf8.RuneCountInString(str) > maxDataText {
				return norm, "too_long"
			}
		}
	case DataNumber:
		var f float64
		if json.Unmarshal(norm, &f) != nil {
			return norm, "invalid"
		}
	case DataBool:
		var b bool
		if json.Unmarshal(norm, &b) != nil {
			return norm, "invalid"
		}
	case DataImage:
		var id int64
		if json.Unmarshal(norm, &id) != nil || id <= 0 {
			return norm, "invalid"
		}
		as, err := s.R.Site.GetAsset(ctx, id)
		if err != nil || !strings.HasPrefix(as.ContentType, "image/") {
			return norm, "invalid"
		}
	case DataURL:
		var str string
		if json.Unmarshal(norm, &str) != nil || len(str) > maxDataText {
			return norm, "invalid"
		}
		u, err := url.Parse(str)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return norm, "invalid"
		}
	case DataPhone:
		var str string
		if json.Unmarshal(norm, &str) != nil || !dataPhone.MatchString(str) {
			return norm, "invalid"
		}
	case DataJSON:
		// any valid JSON within the size limit
	default:
		return norm, "invalid"
	}
	return norm, ""
}

// imageRefs lists the asset ids an item refers to: its image value, or the
// image cells of rows.
func imageRefs(it DataItem, rows []DataRow) []int64 {
	var ids []int64
	add := func(raw json.RawMessage) {
		if id, err := strconv.ParseInt(string(raw), 10, 64); err == nil && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if it.Kind == DataValue {
		if it.ValueType == DataImage {
			add(it.Value)
		}
		return ids
	}
	for _, c := range it.Columns {
		if c.Type != DataImage {
			continue
		}
		for _, r := range rows {
			add(r.Cells[c.Name])
		}
	}
	return ids
}

// Patch changes the description, visibility, value or columns of an item.
func (s *DataService) Patch(ctx context.Context, key string, in DataInput, a Actor) (DataItem, error) {
	var it DataItem
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if it, err = s.R.Data.GetByKey(ctx, key); err != nil {
			return err
		}
		if it.Owner != "" && (in.Description != nil || in.ValueType != nil || in.Value != nil || in.Columns != nil) {
			return ErrOwned
		}
		v := validation{}
		var changed []string
		if in.Description != nil {
			it.Description = strings.TrimSpace(*in.Description)
			changed = append(changed, "description")
		}
		oldVis := it.Visibility
		if in.Visibility != nil {
			if !in.Visibility.Valid() {
				v.add("visibility", "invalid")
			}
			it.Visibility = *in.Visibility
		}
		if in.ValueType != nil || in.Value != nil {
			if it.Kind != DataValue {
				v.add("valueType", "invalid")
			} else {
				if in.ValueType != nil {
					it.ValueType = *in.ValueType
				}
				it.Value = in.Value
				changed = append(changed, "value")
			}
		}
		oldCols := it.Columns
		if in.Columns != nil {
			if it.Kind != DataTable {
				v.add("columns", "invalid")
			} else {
				it.Columns = *in.Columns
				if it.Columns == nil {
					it.Columns = []DataColumn{}
				}
				changed = append(changed, "columns")
			}
		}
		if err := v.err(); err != nil {
			return err
		}
		if err := s.check(ctx, &it); err != nil {
			return err
		}
		if in.Columns != nil && it.Kind == DataTable {
			if err := s.applyColumns(ctx, it, oldCols); err != nil {
				return err
			}
		}
		it.UpdatedBy, it.UpdatedAt = a.UserID, s.Now()
		if err := s.R.Data.Update(ctx, &it); err != nil {
			return err
		}
		if err := s.refreshRefs(ctx, it); err != nil {
			return err
		}
		if len(changed) > 0 {
			if err := s.audit(ctx, a, ActDataUpdated, it.Key, map[string]any{"fields": changed}); err != nil {
				return err
			}
		}
		if it.Visibility != oldVis {
			return s.audit(ctx, a, ActDataVisibilitySet, it.Key, map[string]any{"from": string(oldVis), "to": string(it.Visibility)})
		}
		return nil
	})
	return it, err
}

// applyColumns refuses a type change on a non-empty table and drops the cells
// of removed columns.
func (s *DataService) applyColumns(ctx context.Context, it DataItem, old []DataColumn) error {
	rows, err := s.R.Data.Rows(ctx, it.ID)
	if err != nil || len(rows) == 0 {
		return err
	}
	keep := map[string]bool{}
	for _, c := range it.Columns {
		keep[c.Name] = true
		for _, o := range old {
			if o.Name == c.Name && o.Type != c.Type {
				return &ValidationError{Fields: map[string]string{"columns": "type_change_not_empty"}}
			}
		}
	}
	var dirty []DataRow
	for _, r := range rows {
		drop := false
		for name := range r.Cells {
			if !keep[name] {
				delete(r.Cells, name)
				drop = true
			}
		}
		if drop {
			dirty = append(dirty, r)
		}
	}
	return s.R.Data.PutRows(ctx, it.ID, dirty)
}

func (s *DataService) refreshRefs(ctx context.Context, it DataItem) error {
	var rows []DataRow
	if it.Kind == DataTable {
		var err error
		if rows, err = s.R.Data.Rows(ctx, it.ID); err != nil {
			return err
		}
	}
	return s.R.Data.SetAssetRefs(ctx, it.ID, imageRefs(it, rows))
}

// Delete removes an item with its rows.
func (s *DataService) Delete(ctx context.Context, key string, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		it, err := s.R.Data.GetByKey(ctx, key)
		if err != nil {
			return err
		}
		if it.Owner != "" {
			return ErrOwned
		}
		if err := s.R.Data.Delete(ctx, it.ID); err != nil {
			return err
		}
		return s.audit(ctx, a, ActDataDeleted, key, map[string]any{"kind": string(it.Kind)})
	})
}

func (s *DataService) table(ctx context.Context, key string) (DataItem, error) {
	it, err := s.R.Data.GetByKey(ctx, key)
	if err == nil && it.Kind != DataTable {
		err = ErrNotFound
	}
	if err == nil && it.Owner != "" {
		err = ErrOwned
	}
	return it, err
}

// UpsertRows adds rows without an ID and changes the others; it returns all
// rows of the table after the change.
func (s *DataService) UpsertRows(ctx context.Context, key string, rows []DataRow, a Actor) ([]DataRow, error) {
	var out []DataRow
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		it, err := s.table(ctx, key)
		if err != nil {
			return err
		}
		v := validation{}
		cols := map[string]DataType{}
		for _, c := range it.Columns {
			cols[c.Name] = c.Type
		}
		added := 0
		for i := range rows {
			if rows[i].ID == 0 {
				added++
			}
			for name, raw := range rows[i].Cells {
				t, ok := cols[name]
				if !ok {
					v.add("cells."+name, "invalid")
					continue
				}
				norm, reason := s.checkValue(ctx, t, raw)
				if reason != "" {
					v.add("cells."+name, reason)
				}
				rows[i].Cells[name] = norm
			}
		}
		if it.RowCount+added > maxDataRows {
			v.add("rows", "too_many")
		}
		if err := v.err(); err != nil {
			return err
		}
		if err := s.R.Data.PutRows(ctx, it.ID, rows); err != nil {
			return err
		}
		if out, err = s.R.Data.Rows(ctx, it.ID); err != nil {
			return err
		}
		if err := s.touch(ctx, &it, a); err != nil {
			return err
		}
		if err := s.R.Data.SetAssetRefs(ctx, it.ID, imageRefs(it, out)); err != nil {
			return err
		}
		return s.audit(ctx, a, ActDataRowsChanged, key, map[string]any{"upserted": len(rows), "deleted": 0})
	})
	return out, err
}

// RowPatch changes one table row: with ID only the given cells (and the
// position, when set) change; without ID it is a new row, last unless
// Position is set.
type RowPatch struct {
	ID       *int64
	Position *int
	Cells    map[string]json.RawMessage
}

// PatchRows merges patches into the current rows and saves them in one
// transaction, so a concurrent edit of other cells is not overwritten with a
// stale copy. It returns all rows of the table after the change.
func (s *DataService) PatchRows(ctx context.Context, key string, patches []RowPatch, a Actor) ([]DataRow, error) {
	var out []DataRow
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		it, err := s.table(ctx, key)
		if err != nil {
			return err
		}
		current, err := s.R.Data.Rows(ctx, it.ID)
		if err != nil {
			return err
		}
		byID := make(map[int64]DataRow, len(current))
		next := 0
		for _, r := range current {
			byID[r.ID] = r
			next = max(next, r.Position+1)
		}
		rows := make([]DataRow, 0, len(patches))
		for _, p := range patches {
			row := DataRow{Cells: map[string]json.RawMessage{}}
			if p.ID != nil {
				old, ok := byID[*p.ID]
				if !ok {
					return &ValidationError{Fields: map[string]string{"rows": "unknown_id"}, Details: map[string]string{"id": strconv.FormatInt(*p.ID, 10)}}
				}
				row.ID, row.Position = old.ID, old.Position
				for k, c := range old.Cells {
					row.Cells[k] = c
				}
			} else {
				row.Position = next
				next++
			}
			if p.Position != nil {
				row.Position = *p.Position
			}
			for k, c := range p.Cells {
				row.Cells[k] = c
			}
			rows = append(rows, row)
		}
		out, err = s.UpsertRows(ctx, key, rows, a)
		return err
	})
	return out, err
}

func (s *DataService) touch(ctx context.Context, it *DataItem, a Actor) error {
	it.UpdatedBy, it.UpdatedAt = a.UserID, s.Now()
	return s.R.Data.Update(ctx, it)
}

// DeleteRows removes rows of a table.
func (s *DataService) DeleteRows(ctx context.Context, key string, ids []int64, a Actor) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		it, err := s.table(ctx, key)
		if err != nil {
			return err
		}
		if err := s.R.Data.DeleteRows(ctx, it.ID, ids); err != nil {
			return err
		}
		if err := s.touch(ctx, &it, a); err != nil {
			return err
		}
		if err := s.refreshRefs(ctx, it); err != nil {
			return err
		}
		return s.audit(ctx, a, ActDataRowsChanged, key, map[string]any{"upserted": 0, "deleted": len(ids)})
	})
}

func (s *DataService) allowPublic(ip string) bool {
	return s.Limits.Allow("pubdata:"+ip, publicLimit, publicWindow)
}

// Public returns every public item with its rows, rate-limited per IP.
func (s *DataService) Public(ctx context.Context, ip string) ([]DataView, error) {
	if !s.allowPublic(ip) {
		return nil, ErrRateLimited
	}
	return s.views(ctx, VisPublic)
}

// CanReadResidents reports whether u reads residents-level data: an active
// resident or an active admin.
func CanReadResidents(u User) bool {
	return u.Status == StatusActive && (u.IsResident || u.IsAdmin)
}

// ForUser returns the items u may read outside the admin panel: public and,
// for a resident or an admin, residents items. It is rate-limited per user
// like Public is per IP.
func (s *DataService) ForUser(ctx context.Context, u User) ([]DataView, error) {
	if !s.Limits.Allow("medata:"+strconv.FormatInt(u.ID, 10), publicLimit, publicWindow) {
		return nil, ErrRateLimited
	}
	if CanReadResidents(u) {
		return s.views(ctx, VisPublic, VisResidents)
	}
	return s.views(ctx, VisPublic)
}

func (s *DataService) views(ctx context.Context, only ...Visibility) ([]DataView, error) {
	items, err := s.R.Data.List(ctx, only...)
	if err != nil {
		return nil, err
	}
	off, err := s.Modules.DisabledOwners(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DataView, 0, len(items))
	for _, it := range items {
		if off[it.Owner] {
			continue // a module that is off: its data is kept but not served
		}
		v, err := s.view(ctx, it)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// PublicOne returns one public item; a private or missing one is ErrNotFound.
func (s *DataService) PublicOne(ctx context.Context, key, ip string) (DataView, error) {
	if !s.allowPublic(ip) {
		return DataView{}, ErrRateLimited
	}
	it, err := s.R.Data.GetByKey(ctx, key)
	if err != nil {
		return DataView{}, err
	}
	if it.Visibility != VisPublic {
		return DataView{}, ErrNotFound
	}
	if it.Owner != "" {
		off, err := s.Modules.DisabledOwners(ctx)
		if err != nil {
			return DataView{}, err
		}
		if off[it.Owner] {
			return DataView{}, ErrNotFound
		}
	}
	return s.view(ctx, it)
}

// AssetIsPublic reports whether anyone may download the asset.
func (s *DataService) AssetIsPublic(ctx context.Context, assetID int64) (bool, error) {
	public, _, err := s.AssetAccess(ctx, assetID)
	return public, err
}

// AssetAccess says how the asset may be served outside the admin panel:
// public when a public item of a module that is on (or of no module) refers
// to it; blocked when items refer to it and all of them belong to modules
// that are off, so not even a signed URL serves it.
func (s *DataService) AssetAccess(ctx context.Context, assetID int64) (public, blocked bool, err error) {
	refs, err := s.R.Data.AssetRefs(ctx, assetID)
	if err != nil || len(refs) == 0 {
		return false, false, err
	}
	off, err := s.Modules.DisabledOwners(ctx)
	if err != nil {
		return false, false, err
	}
	blocked = true
	for _, r := range refs {
		if off[r.Owner] {
			continue
		}
		blocked = false
		public = public || r.Visibility == VisPublic
	}
	return public, blocked, nil
}

// systemActor is who a module's own writes are recorded as.
var systemActor = Actor{}

// EnsureOwnedTable creates a module's table item when it is missing, with the
// given visibility, or brings the description and columns of an existing
// one up to date (its visibility is the admins' choice and stays).
func (s *DataService) EnsureOwnedTable(ctx context.Context, owner, key, description string, cols []DataColumn, vis Visibility) (DataItem, error) {
	var it DataItem
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		it, err = s.R.Data.GetByKey(ctx, key)
		switch {
		case errors.Is(err, ErrNotFound):
			now := s.Now()
			it = DataItem{Key: key, Kind: DataTable, Description: description, Visibility: vis, Columns: cols, Owner: owner,
				CreatedAt: now, UpdatedAt: now}
			if err := s.R.Data.Create(ctx, &it); err != nil {
				return err
			}
			return s.audit(ctx, systemActor, ActDataCreated, key, map[string]any{"kind": string(DataTable),
				"visibility": string(vis), "owner": owner})
		case err != nil:
			return err
		case it.Owner != owner || it.Kind != DataTable:
			return ErrOwned
		}
		if it.Description == description && slices.Equal(it.Columns, cols) {
			return nil
		}
		old := it.Columns
		it.Description, it.Columns, it.UpdatedAt = description, cols, s.Now()
		if err := s.applyColumns(ctx, it, old); err != nil {
			return err
		}
		return s.R.Data.Update(ctx, &it)
	})
	return it, err
}

// ReplaceOwnedRows replaces every row of a module's table. Module code is
// trusted: the cells are not checked like an admin's.
func (s *DataService) ReplaceOwnedRows(ctx context.Context, owner, key string, rows []DataRow) error {
	return s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		it, err := s.R.Data.GetByKey(ctx, key)
		if err != nil {
			return err
		}
		if it.Owner != owner || it.Kind != DataTable {
			return ErrOwned
		}
		old, err := s.R.Data.Rows(ctx, it.ID)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(old))
		for _, r := range old {
			ids = append(ids, r.ID)
		}
		if err := s.R.Data.DeleteRows(ctx, it.ID, ids); err != nil {
			return err
		}
		fresh := make([]DataRow, len(rows))
		for i, r := range rows {
			fresh[i] = DataRow{Position: i, Cells: r.Cells}
		}
		if err := s.R.Data.PutRows(ctx, it.ID, fresh); err != nil {
			return err
		}
		it.UpdatedBy, it.UpdatedAt = nil, s.Now()
		if err := s.R.Data.Update(ctx, &it); err != nil {
			return err
		}
		return s.R.Data.SetAssetRefs(ctx, it.ID, imageRefs(it, fresh))
	})
}

// AssetURLTTL is about how long a signed asset URL works: it expires at the
// start of the hour after next, so it lasts 1–2 hours and stays the same for
// an hour, which lets browsers cache the image.
const AssetURLTTL = time.Hour

const assetURLSalt = "data-asset:"

func (s *DataService) assetSig(id, exp string) string {
	m := hmac.New(sha256.New, s.Secret)
	m.Write([]byte(assetURLSalt + id + "." + exp))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// SignedAssetURL returns an expiring URL of the asset that works without a
// cookie: a site page's frame has none, yet must show residents-level images.
func (s *DataService) SignedAssetURL(id int64) string {
	ids := strconv.FormatInt(id, 10)
	exp := strconv.FormatInt(s.Now().Truncate(AssetURLTTL).Add(2*AssetURLTTL).Unix(), 10)
	return "/api/v1/assets/" + ids + "?exp=" + exp + "&sig=" + s.assetSig(ids, exp)
}

// ValidAssetSig reports whether exp and sig of a SignedAssetURL name asset id
// and have not expired.
func (s *DataService) ValidAssetSig(id int64, exp, sig string) bool {
	e, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || len(s.Secret) == 0 {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(s.assetSig(strconv.FormatInt(id, 10), exp))) && s.Now().Before(time.Unix(e, 0))
}
