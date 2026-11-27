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

// Data types; url and phone are for columns only.
const (
	DataText          DataType = "text"
	DataLocalizedText DataType = "localized_text"
	DataNumber        DataType = "number"
	DataBool          DataType = "bool"
	DataImage         DataType = "image"
	DataJSON          DataType = "json"
	DataURL           DataType = "url"   // columns only
	DataPhone         DataType = "phone" // columns only
)

// DataColumn is one column of a table item.
type DataColumn struct {
	Name  string   `json:"name"`
	Label string   `json:"label"`
	Type  DataType `json:"type"`
}

// DataItem is a named value or table in the data space.
type DataItem struct {
	ID          int64
	Key         string
	Kind        DataKind
	Description string
	Public      bool
	ValueType   DataType        // value items
	Value       json.RawMessage // value items; "null" when empty
	Columns     []DataColumn    // table items
	RowCount    int             // table items, filled on read
	CreatedBy   *int64
	UpdatedBy   *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	List(ctx context.Context, publicOnly bool) ([]DataItem, error)
	Rows(ctx context.Context, itemID int64) ([]DataRow, error)
	PutRows(ctx context.Context, itemID int64, rows []DataRow) error
	DeleteRows(ctx context.Context, itemID int64, ids []int64) error
	SetAssetRefs(ctx context.Context, itemID int64, assetIDs []int64) error
	AssetIsPublic(ctx context.Context, assetID int64) (bool, error)
}
