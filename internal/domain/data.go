package domain

import (
	"context"
	"encoding/json"
	"time"
)

// DataKind says whether a data item holds one value or a table.
type DataKind string

// Data kinds.
const (
	DataValue DataKind = "value"
	DataTable DataKind = "table"
)

// DataType is the type of a value item or of a table column.
type DataType string

// Data types of values and table columns (json: values only).
const (
	DataText          DataType = "text"
	DataLocalizedText DataType = "localized_text"
	DataNumber        DataType = "number"
	DataBool          DataType = "bool"
	DataImage         DataType = "image"
	DataJSON          DataType = "json"
	DataURL           DataType = "url"
	DataPhone         DataType = "phone"
)

// DataColumn is one column of a table item.
type DataColumn struct {
	Name  string   `json:"name"`
	Label string   `json:"label"`
	Type  DataType `json:"type"`
}

// Visibility says who may read a data item outside the admin panel and MCP.
type Visibility string

// Visibility levels. Admins read every item in the panel and through MCP;
// VisAdmins items are never served anywhere else.
const (
	VisPublic    Visibility = "public"    // anyone, through /public/data
	VisResidents Visibility = "residents" // signed-in residents and admins, through /me/data
	VisAdmins    Visibility = "admins"    // only admins
)

// Valid reports whether v is one of the levels.
func (v Visibility) Valid() bool { return v == VisPublic || v == VisResidents || v == VisAdmins }

// DataItem is a named value or table in the data space.
type DataItem struct {
	ID          int64
	Key         string
	Kind        DataKind
	Description string
	Visibility  Visibility
	ValueType   DataType        // value items
	Value       json.RawMessage // value items; "null" when empty
	Columns     []DataColumn    // table items
	RowCount    int             // table items, filled on read
	// Owner is the module that writes the item ("module:feed"); "" for items
	// admins keep. An owned item is read-only except its visibility.
	Owner     string
	CreatedBy *int64
	UpdatedBy *int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DataRow is one row of a table item; Cells maps column names to JSON values.
type DataRow struct {
	ID       int64
	Position int
	Cells    map[string]json.RawMessage
}

// DataRepo stores the data space.
type DataRepo interface {
	Create(ctx context.Context, it *DataItem) error
	Update(ctx context.Context, it *DataItem) error
	Delete(ctx context.Context, id int64) error
	GetByKey(ctx context.Context, key string) (DataItem, error)
	// List returns items ordered by key: those with one of the given
	// visibilities, or every item when only is empty.
	List(ctx context.Context, only ...Visibility) ([]DataItem, error)
	Rows(ctx context.Context, itemID int64) ([]DataRow, error)
	PutRows(ctx context.Context, itemID int64, rows []DataRow) error
	DeleteRows(ctx context.Context, itemID int64, ids []int64) error
	SetAssetRefs(ctx context.Context, itemID int64, assetIDs []int64) error
	AssetIsPublic(ctx context.Context, assetID int64) (bool, error)
	AssetRefs(ctx context.Context, assetID int64) ([]AssetRef, error)
}

// AssetRef is an item that refers to an asset, as far as serving it matters.
type AssetRef struct {
	Visibility Visibility
	Owner      string
}
