package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/igorrius/homelander-bot/internal/domain"
)

const (
	defaultRowLimit = 100
	maxRowLimit     = 200
)

// itemOut is a data item as tools show it. Value is the decoded JSON value.
type itemOut struct {
	Key         string              `json:"key"`
	Kind        string              `json:"kind" jsonschema:"value or table"`
	Description string              `json:"description"`
	Public      bool                `json:"public" jsonschema:"public items are readable by anyone, including the home page"`
	ValueType   string              `json:"value_type,omitempty"`
	Value       any                 `json:"value,omitempty"`
	Columns     []domain.DataColumn `json:"columns,omitempty"`
	RowCount    *int                `json:"row_count,omitempty"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

func toItem(it domain.DataItem, withValue bool) itemOut {
	o := itemOut{Key: it.Key, Kind: string(it.Kind), Description: it.Description, Public: it.Public, UpdatedAt: it.UpdatedAt}
	switch it.Kind {
	case domain.DataValue:
		o.ValueType = string(it.ValueType)
		if withValue && len(it.Value) > 0 {
			_ = json.Unmarshal(it.Value, &o.Value)
		}
	case domain.DataTable:
		o.Columns = it.Columns
		n := it.RowCount
		o.RowCount = &n
	}
	return o
}

type rowOut struct {
	ID       int64          `json:"id"`
	Position int            `json:"position"`
	Cells    map[string]any `json:"cells"`
}

func toRows(rows []domain.DataRow) []rowOut {
	out := make([]rowOut, 0, len(rows))
	for _, r := range rows {
		cells := make(map[string]any, len(r.Cells))
		for k, v := range r.Cells {
			var x any
			_ = json.Unmarshal(v, &x)
			cells[k] = x
		}
		out = append(out, rowOut{r.ID, r.Position, cells})
	}
	return out
}

func rawJSON(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, inputError("value is not JSON")
	}
	return b, nil
}

type (
	noArgs     struct{}
	catalogOut struct {
		Items []itemOut `json:"items"`
	}
	getIn struct {
		Key    string `json:"key"`
		Offset int    `json:"offset,omitempty" jsonschema:"first row to return (tables)"`
		Limit  int    `json:"limit,omitempty" jsonschema:"rows to return, 1..200, default 100 (tables)"`
	}
	getOut struct {
		Item  itemOut  `json:"item"`
		Rows  []rowOut `json:"rows,omitempty"`
		Total *int     `json:"total,omitempty" jsonschema:"number of rows in the table"`
	}
	setValueIn struct {
		Key         string  `json:"key" jsonschema:"lower-case path such as house.address: segments of a-z, 0-9 and _, joined by dots"`
		ValueType   *string `json:"value_type,omitempty" jsonschema:"text, localized_text, number, bool, image, url, phone or json; required when the value is new"`
		Value       any     `json:"value" jsonschema:"the value: a string for text, {\"uk\":...,\"en\":...} for localized_text, a number, a bool, an asset id for image, any JSON for json"`
		Description *string `json:"description,omitempty"`
		Public      *bool   `json:"public,omitempty"`
	}
	createTableIn struct {
		Key         string              `json:"key"`
		Columns     []domain.DataColumn `json:"columns" jsonschema:"ordered columns; type is text, localized_text, number, bool, image, url or phone"`
		Description *string             `json:"description,omitempty"`
		Public      *bool               `json:"public,omitempty"`
	}
	columnsIn struct {
		Key     string              `json:"key"`
		Columns []domain.DataColumn `json:"columns" jsonschema:"the full new column list; removed columns lose their cells; a type changes only on an empty table"`
	}
	rowIn struct {
		ID       *int64         `json:"id,omitempty" jsonschema:"an existing row to change; omit to add a row"`
		Position *int           `json:"position,omitempty" jsonschema:"sort position; omit to keep (existing) or append (new)"`
		Cells    map[string]any `json:"cells" jsonschema:"column name to value; for an existing row only the given columns change"`
	}
	rowsIn struct {
		Key  string  `json:"key"`
		Rows []rowIn `json:"rows"`
	}
	rowsOut struct {
		Rows []rowOut `json:"rows"`
	}
	rowsDeleteIn struct {
		Key string  `json:"key"`
		IDs []int64 `json:"ids"`
	}
	publicIn struct {
		Key    string `json:"key"`
		Public bool   `json:"public"`
	}
	keyIn struct {
		Key string `json:"key"`
	}
	doneOut struct {
		OK bool `json:"ok"`
	}
	assetIn struct {
		Filename string `json:"filename" jsonschema:"for your reference only; the type is detected from the bytes"`
		Base64   string `json:"base64" jsonschema:"the file, standard base64"`
	}
	assetOut struct {
		ID  int64  `json:"id" jsonschema:"use as the value of an image value or cell"`
		URL string `json:"url" jsonschema:"served publicly once a public item refers to the asset"`
	}
)

// handler adapts a tool body that needs the actor; domain errors become
// readable tool errors.
func handler[In, Out any](t tools, fn func(context.Context, domain.Actor, In) (Out, error)) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var zero Out
		a, err := actor(req)
		if err != nil {
			return nil, zero, err
		}
		out, err := fn(ctx, a, in)
		if err != nil {
			return nil, zero, t.toolError(err)
		}
		return nil, out, nil
	}
}

func (t tools) addData(s *mcp.Server) {
	d := t.d
	mcp.AddTool(s, &mcp.Tool{Name: "data_catalog", Description: "List every data item (values and tables) with its key, kind, type or columns, public flag, description and row count. Start here to see what data exists."},
		handler(t, func(ctx context.Context, _ domain.Actor, _ noArgs) (catalogOut, error) {
			items, err := d.Data.List(ctx)
			out := catalogOut{Items: make([]itemOut, 0, len(items))}
			for _, it := range items {
				out.Items = append(out.Items, toItem(it, false))
			}
			return out, err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_get", Description: "Read one data item: a value with its content, or a table with a page of rows (offset, limit up to 200)."},
		handler(t, func(ctx context.Context, _ domain.Actor, in getIn) (getOut, error) {
			if in.Limit == 0 {
				in.Limit = defaultRowLimit
			}
			if in.Limit < 1 || in.Limit > maxRowLimit || in.Offset < 0 {
				return getOut{}, inputError("limit must be 1..200 and offset at least 0")
			}
			v, err := d.Data.Get(ctx, in.Key)
			if err != nil {
				return getOut{}, err
			}
			out := getOut{Item: toItem(v.Item, true)}
			if v.Item.Kind == domain.DataTable {
				total := len(v.Rows)
				rows := v.Rows[min(in.Offset, total):min(in.Offset+in.Limit, total)]
				out.Rows, out.Total = toRows(rows), &total
				out.Item.RowCount = &total
			}
			return out, nil
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_set_value", Description: "Create a value item, or change an existing one (value, type, description, public). value_type is required when the key is new."},
		handler(t, func(ctx context.Context, a domain.Actor, in setValueIn) (itemOut, error) {
			raw, err := rawJSON(in.Value)
			if err != nil {
				return itemOut{}, err
			}
			di := domain.DataInput{Key: in.Key, Kind: domain.DataValue, Value: raw, Description: in.Description, Public: in.Public}
			if in.ValueType != nil {
				vt := domain.DataType(*in.ValueType)
				di.ValueType = &vt
			}
			cur, err := d.Data.Get(ctx, in.Key)
			var it domain.DataItem
			switch {
			case errors.Is(err, domain.ErrNotFound):
				if di.ValueType == nil {
					return itemOut{}, inputError("value_type is required for a new value")
				}
				it, err = d.Data.Create(ctx, di, a)
			case err != nil:
				return itemOut{}, err
			case cur.Item.Kind != domain.DataValue:
				return itemOut{}, inputError("%s is a table; use the data_rows_* tools", in.Key)
			default:
				it, err = d.Data.Patch(ctx, in.Key, di, a)
			}
			return toItem(it, true), err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_create_table", Description: "Create a table item with ordered columns. Fill it with data_rows_upsert."},
		handler(t, func(ctx context.Context, a domain.Actor, in createTableIn) (itemOut, error) {
			cols := in.Columns
			if cols == nil {
				cols = []domain.DataColumn{}
			}
			it, err := d.Data.Create(ctx, domain.DataInput{Key: in.Key, Kind: domain.DataTable, Columns: &cols,
				Description: in.Description, Public: in.Public}, a)
			return toItem(it, false), err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_update_table_columns", Description: "Replace the column list of a table. Removed columns lose their cells; a column type can change only while the table is empty."},
		handler(t, func(ctx context.Context, a domain.Actor, in columnsIn) (itemOut, error) {
			cols := in.Columns
			if cols == nil {
				cols = []domain.DataColumn{}
			}
			it, err := d.Data.Patch(ctx, in.Key, domain.DataInput{Columns: &cols}, a)
			return toItem(it, false), err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_rows_upsert", Description: "Add rows (no id) and change rows (with id) of a table in one step; returns all rows. Up to 1000 rows per table."},
		handler(t, func(ctx context.Context, a domain.Actor, in rowsIn) (rowsOut, error) {
			patches := make([]domain.RowPatch, 0, len(in.Rows))
			for _, r := range in.Rows {
				p := domain.RowPatch{ID: r.ID, Position: r.Position, Cells: make(map[string]json.RawMessage, len(r.Cells))}
				for k, c := range r.Cells {
					raw, err := rawJSON(c)
					if err != nil {
						return rowsOut{}, err
					}
					p.Cells[k] = raw
				}
				patches = append(patches, p)
			}
			out, err := d.Data.PatchRows(ctx, in.Key, patches, a)
			return rowsOut{Rows: toRows(out)}, err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_rows_delete", Description: "Delete rows of a table by id."},
		handler(t, func(ctx context.Context, a domain.Actor, in rowsDeleteIn) (doneOut, error) {
			return doneOut{true}, d.Data.DeleteRows(ctx, in.Key, in.IDs, a)
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_set_public", Description: "Make an item public (readable by anyone, including the home page) or private."},
		handler(t, func(ctx context.Context, a domain.Actor, in publicIn) (itemOut, error) {
			it, err := d.Data.Patch(ctx, in.Key, domain.DataInput{Public: &in.Public}, a)
			return toItem(it, false), err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "data_delete", Description: "Delete a data item and its rows. Pages that read the key will no longer find it."},
		handler(t, func(ctx context.Context, a domain.Actor, in keyIn) (doneOut, error) {
			return doneOut{true}, d.Data.Delete(ctx, in.Key, a)
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "asset_upload", Description: "Upload an image (PNG, JPEG, WebP or GIF, up to 5 MB) for image values and cells; returns its asset id."},
		handler(t, func(ctx context.Context, a domain.Actor, in assetIn) (assetOut, error) {
			data, err := base64.StdEncoding.DecodeString(in.Base64)
			if err != nil {
				return assetOut{}, inputError("base64 is not valid standard base64")
			}
			as, err := d.Catalog.UploadImage(ctx, data, a)
			return assetOut{ID: as.ID, URL: "/api/v1/assets/" + strconv.FormatInt(as.ID, 10)}, err
		}))
}
