# Site data space (part 1 of 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admins keep named values and tables (with a public flag) in the system, edit them in the panel, and anyone can read the public ones through a rate-limited, cached, CORS-open endpoint; assets are served only when something public refers to them.

**Architecture:** New `data_items` / `data_rows` / `data_asset_refs` tables (migration `0004`), a `DataRepo` in `internal/storage/sqlite`, a `DataService` in `internal/domain` that validates keys, types, columns and rows and writes the audit log, admin routes under `/api/v1/admin/data`, public routes under `/api/v1/public/data`, and two admin pages in the React app. Parts 2 (pages) and 3 (MCP) get their own plans and build on the interfaces produced here.

**Tech Stack:** Go 1.x stdlib `net/http`, `modernc.org/sqlite`, existing `internal/ratelimit`; React + TypeScript + Mantine + TanStack Query + i18next; vitest.

**Spec:** [docs/superpowers/specs/2026-11-27-site-data-pages-and-mcp-design.md](../specs/2026-11-27-site-data-pages-and-mcp-design.md) — sections 2, 4, 7, 8, 9 (step 1).

## Global Constraints

- Every schema change is a new migration file: `internal/storage/sqlite/migrations/0004_data.sql`.
- Key: `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`, max 100 chars, unique across values and tables; keys are never renamed.
- Value types: `text`, `localized_text`, `number`, `bool`, `image`, `json` (json max 64 KiB).
- Column types: `text`, `localized_text`, `number`, `bool`, `image`, `url`, `phone`; column name follows the key-segment rule `^[a-z][a-z0-9_]*$`.
- Table: max **1000** rows; column type change only on an empty table; removing a column drops its values.
- Public flag defaults to **false**.
- Public responses: `Access-Control-Allow-Origin: *`, `Cache-Control: public, max-age=30`, `ETag`; private or missing → the same `404 not_found`.
- Public rate limit: **120 requests per minute per client IP** (`ClientIP(r, cfg.TrustProxy)`), `429 rate_limited`.
- Every change writes an audit entry with target `data:<key>`; actions `data.created`, `data.updated`, `data.deleted`, `data.public_set`, `data.rows_changed` (one entry per save, details carry counts).
- All new UI text through i18n, uk default + en. Repository is public: no real phones/addresses in fixtures.
- Asset URL path stays `/api/v1/assets/{id}` (spec 4.5 leaves the path to the builder); it now serves only referenced assets.

## Review Focus

1. Item made private again → disappears from `GET /public/data`, its key returns 404, and its images stop being served to guests (test in Tasks 3 and 5).
2. Keys like `Contacts`, `contacts.`, `.x`, `a..b`, `контакти`, 101 chars → `validation` with field `key: invalid` (test in Task 2).
3. Removing a table column → the public output and `Get` no longer contain that cell in any row (test in Task 2).
4. A 1001st row in one upsert batch or across batches → `rows: too_many`, nothing written (test in Task 2).
5. After any change the ETag differs, and a client sending the old `If-None-Match` gets 200 with the new body (test in Task 5).

---

### Task 1: Schema, domain types, SQLite repository

**Files:**
- Create: `internal/storage/sqlite/migrations/0004_data.sql`
- Create: `internal/domain/data.go` (types + `DataRepo` interface only)
- Create: `internal/storage/sqlite/data.go`, `internal/storage/sqlite/data_test.go`
- Modify: `internal/domain/users_service.go` (`Repos.Data`), `internal/storage/sqlite/repos.go`

**Interfaces:**
- Produces (domain):
  ```go
  type DataKind string   // "value" | "table"
  type DataType string   // value types and column types share this type
  type DataColumn struct { Name, Label string; Type DataType }
  type DataItem struct {
      ID int64; Key string; Kind DataKind; Description string; Public bool
      ValueType DataType; Value json.RawMessage   // value items; Value "null" when empty
      Columns []DataColumn; RowCount int            // table items
      CreatedBy, UpdatedBy *int64; CreatedAt, UpdatedAt time.Time
  }
  type DataRow struct { ID int64; Position int; Cells map[string]json.RawMessage }
  type DataRepo interface {
      Create(ctx context.Context, it *DataItem) error              // ErrConflict on taken key
      Update(ctx context.Context, it *DataItem) error              // description, public, value type/value, columns, updated_*
      Delete(ctx context.Context, id int64) error                  // cascades rows and refs
      GetByKey(ctx context.Context, key string) (DataItem, error)  // ErrNotFound
      List(ctx context.Context, publicOnly bool) ([]DataItem, error) // ordered by key, RowCount filled
      Rows(ctx context.Context, itemID int64) ([]DataRow, error)   // ordered by position, id
      PutRows(ctx context.Context, itemID int64, rows []DataRow) error // insert when ID==0 (sets ID), else update; ErrNotFound for a foreign id
      DeleteRows(ctx context.Context, itemID int64, ids []int64) error
      SetAssetRefs(ctx context.Context, itemID int64, assetIDs []int64) error // replaces the item's refs
      AssetIsPublic(ctx context.Context, assetID int64) (bool, error) // site picture OR ref of a public item
  }
  ```
- Produces (sqlite): `sqlite.Data{DB DB}` implementing `domain.DataRepo`; `Repos.Data`.

- [ ] **Step 1: Write the migration.** `data_items(id PK, key TEXT UNIQUE NOT NULL, kind TEXT CHECK IN ('value','table'), description TEXT NOT NULL DEFAULT '', is_public INTEGER 0/1 DEFAULT 0, value_type TEXT, value_json TEXT, columns_json TEXT, created_by/updated_by INTEGER REFERENCES users(id), created_at, updated_at TEXT)`; `data_rows(id PK, item_id REFERENCES data_items ON DELETE CASCADE, position INTEGER, cells_json TEXT NOT NULL)` with index `(item_id, position, id)`; `data_asset_refs(item_id REFERENCES data_items ON DELETE CASCADE, asset_id REFERENCES assets(id), PRIMARY KEY(item_id, asset_id))`. Site picture lives in `settings`; read it the way `Site.Get` does.
- [ ] **Step 2: Write failing repo tests** in `data_test.go`:
  - `TestDataItemCRUD` — create value item, `GetByKey` equals what was stored (`reflect.DeepEqual`), duplicate key → `ErrConflict`, update public+description round-trips, delete → `GetByKey` `ErrNotFound`.
  - `TestDataRows` — table item, `PutRows` two new rows gets IDs, update one by ID, `Rows` ordered by position, `DeleteRows` one, `List` shows `RowCount == 1`; `PutRows` with an ID of another item's row → `ErrNotFound`.
  - `TestDataListPublicOnly` — two items, one public; `List(ctx, true)` returns only it.
  - `TestAssetIsPublic` — asset referenced by a private item → false; item made public → true; site picture asset → true; unreferenced → false; refs removed by `SetAssetRefs(id, nil)` → false.
- [ ] **Step 3: Run** `go test ./internal/storage/sqlite -run 'Data|AssetIsPublic'` — Expected: FAIL (undefined `sqlite.Data`).
- [ ] **Step 4: Implement** `internal/storage/sqlite/data.go` following `labels.go` (scanner, `mapErr`, `fmtTime`, `nullInt`, `boolInt`); columns stored as JSON. Wire `Repos.Data`.
- [ ] **Step 5: Run** `go test ./internal/storage/...` — Expected: PASS (incl. `migrate_test`).
- [ ] **Step 6: Commit** `feat(data): schema and repository for the data space`.

### Task 2: DataService — validation and audit

**Files:**
- Modify: `internal/domain/data.go` (service), `internal/domain/audit.go` (action constants, `DataTarget`)
- Create: `internal/domain/data_service_test.go`; Modify: `internal/domain/testenv_test.go` (`e.data`)

**Interfaces:**
- Consumes: Task 1 types and `DataRepo`.
- Produces:
  ```go
  type DataService struct { R Repos; Now Clock; Limits *ratelimit.Limiter; Locales []string }
  type DataInput struct {               // create and patch; nil = unchanged on patch
      Key string; Kind DataKind; Description *string; Public *bool
      ValueType *DataType; Value json.RawMessage; Columns *[]DataColumn
  }
  type DataView struct { Item DataItem; Rows []DataRow }
  func (s *DataService) List(ctx context.Context) ([]DataItem, error)
  func (s *DataService) Get(ctx context.Context, key string) (DataView, error)
  func (s *DataService) Create(ctx context.Context, in DataInput, a Actor) (DataItem, error)
  func (s *DataService) Patch(ctx context.Context, key string, in DataInput, a Actor) (DataItem, error)
  func (s *DataService) Delete(ctx context.Context, key string, a Actor) error
  func (s *DataService) UpsertRows(ctx context.Context, key string, rows []DataRow, a Actor) ([]DataRow, error) // returns all rows after the change
  func (s *DataService) DeleteRows(ctx context.Context, key string, ids []int64, a Actor) error
  func (s *DataService) Public(ctx context.Context, ip string) ([]DataView, error)          // rate-limited
  func (s *DataService) PublicOne(ctx context.Context, key, ip string) (DataView, error)    // private/missing → ErrNotFound
  func (s *DataService) AssetIsPublic(ctx context.Context, assetID int64) (bool, error)
  const ActDataCreated, ActDataUpdated, ActDataDeleted, ActDataPublicSet, ActDataRowsChanged = "data.created", "data.updated", "data.deleted", "data.public_set", "data.rows_changed"
  func DataTarget(key string) string // "data:" + key
  ```
- Validation reasons (field → reason): `key` invalid/taken; `kind` invalid; `valueType` invalid; `value` invalid/too_long; `columns` invalid (duplicate or bad name, bad type)/type_change_not_empty; `rows` too_many; `cells.<col>` invalid. Description max 1000 runes → `description: too_long`.
- Type rules: `text` string ≤ 2000 runes; `localized_text` object whose keys ⊂ `Locales`, string values ≤ 2000; `number` JSON number; `bool`; `image` integer id of an existing asset with `image/*` content type; `json` any valid JSON ≤ 64 KiB; `url` absolute `http`/`https`; `phone` string matching `^[0-9+() -]{3,30}$`. `null` is always allowed (empty). Unknown cell names → `cells.<name>: invalid`.
- Every write recomputes the item's asset refs (value image id, or all image cells) via `SetAssetRefs`, inside `R.Tx.InTx` together with the change and its audit entry. Patch that only flips `Public` writes `data.public_set` with `{"public": bool}`; other patches write `data.updated`; row changes write `data.rows_changed` with `{"upserted": n, "deleted": m}`.

- [ ] **Step 1: Write failing tests** (`newEnv` gains `e.data = &domain.DataService{R: r, Now: c.Now, Limits: ratelimit.New(c.Now), Locales: []string{"uk","en"}}`):
  - `TestDataKeyValidation` — table-driven: `contacts`, `house.address`, `a1.b_2` accepted; `Contacts`, `contacts.`, `.x`, `a..b`, `контакти`, `1a`, 101×`a` → `key: invalid`; duplicate → `key: taken`.
  - `TestDataValueTypes` — one accepted and one rejected input per value type (e.g. `text` 2001 runes → `value: too_long`; `localized_text` `{"de":"x"}` → `value: invalid`; `image` with a non-image asset → `value: invalid`; `json` 64 KiB+1 → `value: too_long`).
  - `TestDataColumnsAndRows` — create table `contacts` (name text, phone phone); upsert 2 rows; bad phone → `cells.phone: invalid`; unknown cell → `cells.x: invalid`; change column type with rows → `columns: type_change_not_empty`; remove `phone` column → `Get` rows have no `phone` cell; upsert batch making 1001 rows → `rows: too_many` and row count unchanged.
  - `TestDataAudit` — create, patch description, flip public, upsert rows, delete → audit actions in order `data.created, data.updated, data.public_set, data.rows_changed, data.deleted`, all target `data:contacts`, actor = admin id.
  - `TestDataPublic` — private item absent from `Public`, `PublicOne` → `ErrNotFound`; after flip present; 121st `Public` call from the same IP within a minute → `ErrRateLimited`; another IP still allowed.
  - `TestDataAssetRefs` — image value in private item: `AssetIsPublic` false; made public: true; value changed to another asset: old one false.
- [ ] **Step 2: Run** `go test ./internal/domain -run Data` — Expected: FAIL (undefined `DataService`).
- [ ] **Step 3: Implement** the service in `internal/domain/data.go` with the existing `validation` helper and `audit` pattern from `catalog_service.go`. Rate-limit key: `"pubdata:" + ip`, 120 per minute; `Public` and `PublicOne` both count.
- [ ] **Step 4: Run** `go test -race ./internal/domain` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(data): data service with validation, asset refs and audit`.

### Task 3: Asset access rules and data image upload

**Files:**
- Modify: `internal/httpapi/public.go` (`asset` handler, `API.Data`), `internal/httpapi/admin.go` (routes), `cmd/homelander/main.go` (wire `DataService`)
- Create: `internal/httpapi/data.go` (handlers used here and in Task 4), `internal/httpapi/data_test.go`
- Modify: `internal/domain/catalog_service.go` (generalise picture upload into `UploadImage(ctx, kind string, ct string, data []byte, a Actor) (Asset, error)` reusing `pictureTypes` + `image/gif`, 5 MiB)

**Interfaces:**
- Consumes: `DataService.AssetIsPublic`.
- Produces: `GET /api/v1/assets/{id}` serves only when `AssetIsPublic` (else `404 not_found`); `GET /api/v1/admin/assets/{id}` serves any asset to admins; `POST /api/v1/admin/data/assets` (multipart field `file`) → `201 {"id": n, "url": "/api/v1/admin/assets/n"}`; `httpapi.API.Data *domain.DataService`.

- [ ] **Step 1: Write failing tests** in `data_test.go` (extend `newAPIEnv` with `Data`):
  - `TestAssetAccess` — upload via admin endpoint; guest `GET /api/v1/assets/{id}` → 404; admin `GET /api/v1/admin/assets/{id}` → 200 with the bytes; set it as image value of a public item → guest 200; make item private → guest 404; site picture still served to a guest (existing `SitePage` behaviour).
  - `TestDataAssetUpload` — `text/plain` → 422 `file: invalid`; > 5 MiB → 413 or 422 as the existing picture upload returns; guest → 401.
- [ ] **Step 2: Run** `go test ./internal/httpapi -run 'Asset'` — Expected: FAIL.
- [ ] **Step 3: Implement**; keep the existing headers on asset responses (`nosniff`, `CSP: default-src 'none'; sandbox`, `Cache-Control: public, max-age=3600` for public, `private, no-store` for the admin route). Audit for data image upload: none (the value change that references it is audited).
- [ ] **Step 4: Run** `go test -race ./internal/...` — Expected: PASS (site picture tests unchanged).
- [ ] **Step 5: Commit** `feat(data): serve only referenced assets; admin asset route and image upload`.

### Task 4: Admin data API

**Files:**
- Modify: `internal/httpapi/data.go`, `internal/httpapi/admin.go` (register via `registerData(h, routes)` like `registerChats`), `internal/httpapi/data_test.go`

**Interfaces:**
- Consumes: Task 2 service.
- Produces (all `RequireAdmin`, JSON camelCase):
  - `GET /api/v1/admin/data` → `[]itemDTO` (no rows): `{key, kind, description, public, valueType?, value?, columns?, rowCount, updatedAt, updatedBy?: {id, name}}`
  - `POST /api/v1/admin/data` → 201 `itemDTO`; body `{key, kind, description, public, valueType, value, columns}`
  - `GET /api/v1/admin/data/{key}` → `{item: itemDTO, rows: [{id, position, cells}]}`
  - `PATCH /api/v1/admin/data/{key}` → `itemDTO`; any subset of `description, public, valueType+value, columns`
  - `DELETE /api/v1/admin/data/{key}` → 204
  - `PUT /api/v1/admin/data/{key}/rows` body `{rows: [{id?, position, cells}]}` → `{rows: [...]}` (all rows after the change)
  - `DELETE /api/v1/admin/data/{key}/rows/{rowId}` → 204
  - Image values/cells in admin DTOs are the asset id; the web app builds `/api/v1/admin/assets/{id}`.

- [ ] **Step 1: Write failing tests** `TestAdminDataCRUD` (create value → list → get → patch → delete, 404 afterwards), `TestAdminDataRows` (create table, PUT two rows, DELETE one, GET shows one), `TestAdminDataValidation` (bad key → 422 with `fields.key == "invalid"`), `TestAdminDataRequiresAdmin` (resident session → 403, guest → 401).
- [ ] **Step 2: Run** `go test ./internal/httpapi -run AdminData` — Expected: FAIL.
- [ ] **Step 3: Implement** handlers with `decode`, `h.actor(r)`, `h.fail`, `WriteJSON` as in `chats.go`.
- [ ] **Step 4: Run** `go test -race ./internal/httpapi` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(data): admin API for values and tables`.

### Task 5: Public data endpoint

**Files:**
- Modify: `internal/httpapi/data.go`, `internal/httpapi/public.go` (routes), `internal/httpapi/data_test.go`

**Interfaces:**
- Produces:
  - `GET /api/v1/public/data` → `{"<key>": publicDTO, ...}`
  - `GET /api/v1/public/data/{key}` → `publicDTO`
  - `publicDTO`: value → `{"kind":"value","type":T,"value":V}`; table → `{"kind":"table","columns":[{name,label,type}],"rows":[{"id":n,"<col>":v,...}]}`; image ids replaced by `"/api/v1/assets/<id>"`; no description, no author.
  - Headers on 200 and 304: `Access-Control-Allow-Origin: *`, `Cache-Control: public, max-age=30`, `ETag: "<sha256 hex of body, first 32 chars>"`; `If-None-Match` equal → 304 empty body. 404 and 429 also carry `Access-Control-Allow-Origin: *`.

- [ ] **Step 1: Write failing tests** `TestPublicData` (shape for one value, one table with an image cell turned into a URL; private item absent; `/public/data/private.key` → 404 identical to `/public/data/missing`), `TestPublicDataCaching` (ETag present; same ETag → 304; after `PATCH` the old ETag → 200 and new ETag), `TestPublicDataCORS` (header present on 200/404/429), `TestPublicDataRateLimit` (121 requests from one IP → last is 429).
- [ ] **Step 2: Run** `go test ./internal/httpapi -run PublicData` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(data): public data endpoint with CORS, ETag and rate limit`.

### Task 6: Web — Data list page, API hooks, navigation

**Files:**
- Create: `web/src/api/data.ts`, `web/src/pages/admin/DataPage.tsx`, `web/src/pages/admin/DataPage.test.tsx`, `web/src/pages/admin/DataCreateModal.tsx`
- Modify: `web/src/api/types.ts`, `web/src/App.tsx` (route `data` and `data/:key`), `web/src/pages/admin/AdminLayout.tsx` (nav item in the group with Site, icon `IconDatabase`), `web/src/locales/uk.json`, `web/src/locales/en.json` (`admin.nav.data`, `admin.data.*`, `audit.action.data.*`), `web/src/test/fixtures.ts`

**Interfaces:**
- Produces (TS): `DataKind`, `DataType`, `DataColumn`, `DataItem`, `DataRow` in `types.ts` mirroring Task 4 DTOs; hooks `useDataItems()`, `useDataItem(key)`, `useCreateDataItem()`, `usePatchDataItem(key)`, `useDeleteDataItem()`, `usePutDataRows(key)`, `useDeleteDataRow(key)`, `useUploadDataImage()`; query keys `['admin','data']` and `['admin','data',key]`.

- [ ] **Step 1: Write failing tests** in `DataPage.test.tsx`: lists items (key, kind label, public badge, row count for tables); create modal sends `POST /admin/data` with `{key:'house.address', kind:'value', valueType:'text', public:false, ...}` and navigates to `/admin/data/house.address`; 422 `key: invalid` shows `uk.validation.invalid` under the key field; nav link "Дані" present.
- [ ] **Step 2: Run** `cd web && npx vitest run DataPage` — Expected: FAIL.
- [ ] **Step 3: Implement** in the style of `LabelsPage` / `LabelsModal`; create modal: key, kind (value/table), value type (for value) or an initial single `name: text` column (for table), description, public switch.
- [ ] **Step 4: Run** `cd web && npm test && npm run lint && npm run typecheck` — Expected: PASS (`locales.test.ts` checks uk/en key parity).
- [ ] **Step 5: Commit** `feat(web): data list page and API hooks`.

### Task 7: Web — Data item page (value and table editors)

**Files:**
- Create: `web/src/pages/admin/DataItemPage.tsx`, `web/src/pages/admin/DataItemPage.test.tsx`, `web/src/pages/admin/DataValueEditor.tsx`, `web/src/pages/admin/DataTableEditor.tsx`
- Modify: locales

**Interfaces:**
- Consumes: Task 6 hooks; `LocalizedTextInputs`, `ConfirmButton`.

- [ ] **Step 1: Write failing tests**:
  - value item `text`: edit and save → `PATCH /admin/data/house.address` body `{valueType:'text', value:'…'}`; `public` switch → `PATCH` `{public:true}`.
  - `json` value with invalid JSON → inline error, no request sent.
  - `image` value: uploading a file calls `POST /admin/data/assets` then `PATCH` with `{valueType:'image', value:<id>}`; preview uses `/api/v1/admin/assets/<id>`.
  - table: add row, fill cells, save → `PUT /admin/data/contacts/rows` with `{rows:[{position:0, cells:{name:'…', phone:'…'}}]}`; delete row (confirm) → `DELETE /admin/data/contacts/rows/7`; move row up changes `position` values in the next PUT.
  - columns editor: add column, remove column (confirm warns values are dropped) → `PATCH` `{columns:[...]}`; type select disabled when the table has rows.
  - delete item (confirm) → `DELETE /admin/data/contacts`, back to `/admin/data`.
  - Public URL hint shown for public items: `/api/v1/public/data/<key>`.
- [ ] **Step 2: Run** `cd web && npx vitest run DataItemPage` — Expected: FAIL.
- [ ] **Step 3: Implement.** Field per type: text → Textarea; localized_text → `LocalizedTextInputs`; number → NumberInput; bool → Switch; image → FileButton + Image preview; json → Textarea (monospace) validated with `JSON.parse`; url/phone → TextInput. Table: Mantine Table with inline inputs, rows saved in one PUT.
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): edit values and tables in the admin panel`.
