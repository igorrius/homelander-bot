package domain

import (
	"bytes"
	"context"
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
// and modules) keep, and the public read of them.
type DataService struct {
	R       Repos
	Now     Clock
	Limits  *ratelimit.Limiter
	Locales []string
}

// DataInput creates an item or, in Patch, changes it; nil fields stay as they are.
type DataInput struct {
	Key         string
	Kind        DataKind
	Description *string
	Public      *bool
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
	valueTypes = []DataType{DataText, DataLocalizedText, DataNumber, DataBool, DataImage, DataJSON}
	colTypes   = []DataType{DataText, DataLocalizedText, DataNumber, DataBool, DataImage, DataURL, DataPhone}
)

func (s *DataService) audit(ctx context.Context, a Actor, action, key string, details map[string]any) error {
	return s.R.Audit.Add(ctx, AuditEntry{At: s.Now(), ActorUserID: a.UserID, Action: action, Target: DataTarget(key), Details: a.AuditDetails(details), IP: a.IP})
}

// List returns every item without rows.
func (s *DataService) List(ctx context.Context) ([]DataItem, error) { return s.R.Data.List(ctx, false) }

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
	}
	now := s.Now()
	it := DataItem{Key: in.Key, Kind: in.Kind, CreatedBy: a.UserID, UpdatedBy: a.UserID, CreatedAt: now, UpdatedAt: now}
	if in.Description != nil {
		it.Description = strings.TrimSpace(*in.Description)
	}
	if in.Public != nil {
		it.Public = *in.Public
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
		return s.audit(ctx, a, ActDataCreated, it.Key, map[string]any{"kind": string(it.Kind), "public": it.Public})
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

// Patch changes the description, public flag, value or columns of an item.
func (s *DataService) Patch(ctx context.Context, key string, in DataInput, a Actor) (DataItem, error) {
	var it DataItem
	err := s.R.Tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if it, err = s.R.Data.GetByKey(ctx, key); err != nil {
			return err
		}
		v := validation{}
		var changed []string
		if in.Description != nil {
			it.Description = strings.TrimSpace(*in.Description)
			changed = append(changed, "description")
		}
		publicChanged := in.Public != nil && *in.Public != it.Public
		if in.Public != nil {
			it.Public = *in.Public
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
			details := map[string]any{"fields": changed}
			if publicChanged {
				details["public"] = it.Public
			}
			return s.audit(ctx, a, ActDataUpdated, it.Key, details)
		}
		if publicChanged {
			return s.audit(ctx, a, ActDataPublicSet, it.Key, map[string]any{"public": it.Public})
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
	items, err := s.R.Data.List(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]DataView, 0, len(items))
	for _, it := range items {
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
	if !it.Public {
		return DataView{}, ErrNotFound
	}
	return s.view(ctx, it)
}

// AssetIsPublic reports whether anyone may download the asset.
func (s *DataService) AssetIsPublic(ctx context.Context, assetID int64) (bool, error) {
	return s.R.Data.AssetIsPublic(ctx, assetID)
}
