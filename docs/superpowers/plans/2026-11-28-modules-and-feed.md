# Base modules and the information feed — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A minimal registry of built-in modules that an admin turns on and off, module-owned data
items, and the first module — the information feed: admin posts shown in the cabinet, sent once to
each Telegram chat/topic reached by the chosen labels (text or photo), and published as the
module-owned table `feed.posts` for the AI site.

**Architecture:** Backend: `ModuleService` (code registry + `modules` table) gates everything a module
serves; `DataService` learns owners (`data_items.owner`), reserved key prefixes and "disabled module
data is not served"; `FeedService` keeps `feed_posts`, publishes through a new
`Notifier.SendToLabels` (destinations of all labels deduplicated) and rewrites `feed.posts`; the outbox
payload carries an image asset id that the worker loads at send time and the Telegram adapter sends
with `sendPhoto`. HTTP: `httpapi/modules.go` (admin modules, cabinet `/modules`), `httpapi/feed.go`
(admin feed, cabinet feed). Front end: an admin "Modules" section and the feed's admin page under
`/admin/modules/feed`, cabinet navigation built from `GET /api/v1/modules`, a feed home card,
`/my/feed` and `/my/feed/:id`, read-only owned items in the Data section.

**Tech Stack:** Go 1.27 (net/http, SQLite via modernc, gotgbot v2 rc.37, MCP go-sdk), React 19,
Mantine 9, TanStack Query 5, react-router 8, vitest.

**Spec:** `docs/superpowers/specs/2026-11-28-resident-cabinet-and-feed-design.md` §6 (with §1–§3, §7, §8).

## Global Constraints

- Docs, comments, commit messages in English; commit messages end with a blank line and
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- UI texts via locale files, uk (default) and en in parity: `web/src/locales/*.json`, `locales/*.json`
  (both have parity tests).
- The only migration is `internal/storage/sqlite/migrations/0009_modules_feed.sql`.
- Public repo: synthetic fixtures only (`+3805000000NN`, "House A", example.com, chat ids like
  `-1001000000001`).
- Module code `feed`; owner string `module:feed`; reserved prefix `feed.`; owned item `feed.posts`
  (columns `id` number, `date` text ISO-8601 UTC, `text` text, `image` image, `url` url), default
  visibility `residents`, rows = latest 20 published non-deleted posts.
- Post body plain text: ≤ 4000 runes, ≤ 1000 runes with an image; required (trimmed, non-empty).
- Telegram copies are never edited or deleted; plain text, no parse mode; button "Open in cabinet" →
  `<PUBLIC_URL>/my/feed/<id>` only when `PUBLIC_URL` is https (otherwise no button).
- Outbox origin `feed:<id>`; finished rows are kept 30 days (`outboxRetention`).
- Audit actions: `module.enabled`, `module.disabled`, `module.settings_saved` (target `module:<code>`),
  `feed.post_created`, `feed.post_published`, `feed.post_updated`, `feed.post_deleted`
  (target `feed_post:<id>`).
- A disabled module: cabinet API and admin feed API answer 404 `module_disabled`; its owned items are
  left out of `/public/data`, `/public/data/{key}`, `/me/data`; their assets are not served on the
  public or signed asset paths; admins still see them in the Data section and MCP.
- Group chats never get login tokens: feed messages link to plain `/my/feed/<id>`.

## Decisions beyond the design (for the PR body)

- Feed admin lives at `/admin/modules/feed` (tabs Posts / Settings), editor at
  `/admin/modules/feed/posts/new` and `/admin/modules/feed/posts/:id`; the nav gets "Modules"
  (settings group) and "Feed" (community group, only when the module is on).
- `GET /api/v1/modules` returns `[{code, nav: [{key, path, labelKey, icon}]}]`; icons are names the
  front end maps (`news` → `IconNews`).
- Draft posts keep their label choice; publishing fixes it. A body is required (no image-only post).
- Cabinet images are served by `GET /api/v1/feed/posts/{id}/image` (session + module on), not by
  signed URLs, so posts older than the 20 in `feed.posts` keep their image.
- Module row rewrites of `feed.posts` are not audited as data changes (the feed action is); creating
  the owned item is audited as `data.created` with `{"owner": "module:feed"}` and the system actor.
- Module writes skip the generic data value checks (trusted code; the post text may exceed the 2000
  rune text-cell limit).
- Signed asset URLs: refused when every item referring to the asset belongs to a disabled module.
- Owned items: `ErrOwned` (`module_owned`, HTTP 409); reserved keys: validation `key: key_reserved`.

## Review Focus

1. A chat that carries both chosen labels, and a forum topic plus its chat carrying different labels,
   must get one row for the chat and one for the topic — domain test with two labels on one chat and
   a third on a topic of it: 2 rows, not 3.
2. A photo post whose asset was deleted before the worker ran must fail permanently with a clear
   error, not retry for 24 h — worker test deletes the asset, expects `failed`, `last_error`
   mentions the asset.
3. Turning the module off must hide `feed.posts` from `/public/data` even when an admin made it
   `public`, and its image from `/api/v1/assets/{id}` — httpapi test.
4. Editing or deleting a published post must rewrite `feed.posts` (deleted post gone, edited text
   updated) and must not enqueue anything — domain test counts outbox rows.
5. Cursor pagination with posts published out of id order (publish #2 before #1) returns each post
   exactly once across pages — sqlite repo test.

---

### Task 1: Migration, module registry and module service

**Files:**
- Create: `internal/storage/sqlite/migrations/0009_modules_feed.sql`
- Create: `internal/domain/module.go`, `internal/domain/module_service.go`
- Create: `internal/storage/sqlite/modules.go`
- Modify: `internal/domain/users_service.go` (`Repos.Modules ModuleRepo`, `Repos.Feed FeedRepo`),
  `internal/storage/sqlite/repos.go`, `internal/domain/audit.go`, `internal/domain/errors.go`
- Test: `internal/storage/sqlite/migrate_test.go`, `internal/storage/sqlite/modules_test.go`,
  `internal/domain/module_service_test.go`

**Interfaces:**
- Produces:
  - `type ModuleDef struct { Code string; NameKey, DescriptionKey string; DefaultEnabled bool; Nav []ModuleNav; Ensure func(ctx context.Context) error }`
  - `type ModuleNav struct { Key, Path, LabelKey, Icon string }`
  - `type ModuleState struct { Code string; Enabled bool; Settings json.RawMessage; UpdatedBy *int64; UpdatedAt time.Time }`
  - `type ModuleRepo interface { Get(ctx, code string) (ModuleState, error) /* ErrNotFound: no row */; Save(ctx, ModuleState) error }`
  - `type ModuleView struct { ModuleDef; Enabled bool; Settings json.RawMessage; UpdatedBy *int64; UpdatedAt *time.Time }`
  - `ModuleService{R Repos; Now Clock; Defs []ModuleDef}` with `List(ctx) ([]ModuleView, error)`,
    `Get(ctx, code) (ModuleView, error)` (unknown code → `ErrNotFound`), `Enabled(ctx, code) (bool, error)`,
    `Require(ctx, code) error` (off → `ErrModuleOff`), `SetEnabled(ctx, code, on bool, a Actor) (ModuleView, error)`,
    `SaveSettings(ctx, code string, raw json.RawMessage, a Actor) error`, `EnsureAll(ctx) error`,
    `Reserved(key string) bool` (key starts with `<code>.` of any def), `DisabledOwners(ctx) (map[string]bool, error)`.
  - `func ModuleOwner(code string) string` → `"module:" + code`; `func ModuleTarget(code string) string` → `"module:" + code`.
  - `ErrModuleOff = errors.New("module_disabled")`, `ErrOwned = errors.New("module_owned")`.
  - Audit consts `ActModuleEnabled`, `ActModuleDisabled`, `ActModuleSettingsSaved`, `ActFeedPostCreated`,
    `ActFeedPostPublished`, `ActFeedPostUpdated`, `ActFeedPostDeleted`; `FeedPostTarget(id int64) string`.

- [ ] **Step 1: Write the migration** — `UPDATE data_items SET key = 'custom.' || key WHERE key LIKE 'feed.%'`;
  `ALTER TABLE data_items ADD COLUMN owner TEXT`; `modules(code TEXT PRIMARY KEY, enabled INTEGER NOT NULL
  CHECK (enabled IN (0,1)), settings TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(settings)), updated_by
  INTEGER REFERENCES users(id) ON DELETE SET NULL, updated_at TEXT NOT NULL) STRICT`; `feed_posts(id, body,
  image_asset_id REFERENCES assets(id) ON DELETE SET NULL, state CHECK IN ('draft','published'),
  published_at, labels TEXT NOT NULL DEFAULT '[]' json_valid, created_by/updated_by REFERENCES users ON
  DELETE SET NULL, created_at, updated_at, deleted_at) STRICT`; index `feed_posts_published(published_at,
  id) WHERE state = 'published' AND deleted_at IS NULL`; index `outbox_origin ON outbox(origin)`.
- [ ] **Step 2: Migration test** `TestMigration0009RenamesFeedKeys`: migrate to 0008, insert `feed.x` and
  `house.y`, apply all; expect keys `custom.feed.x`, `house.y`, `owner` NULL. Run `go test ./internal/storage/sqlite/ -run 0009` → FAIL, then PASS after step 1.
- [ ] **Step 3: Tests for `sqlite.Modules`** (`Get` missing → `ErrNotFound`; `Save` upsert round-trip incl. settings) and for `ModuleService`: default used without a row; `SetEnabled(false)` writes a row and audits `module.disabled` with target `module:feed`; `SetEnabled(true)` calls `Ensure` once and audits `module.enabled`; `Require` gives `ErrModuleOff` when off; unknown code `ErrNotFound`; `Reserved("feed.posts")` true, `Reserved("feedback.x")` false, `Reserved("custom.feed.x")` false; `DisabledOwners` = `{"module:feed": true}` when off.
- [ ] **Step 4: Implement** repo + service (SetEnabled and Ensure in one transaction; `EnsureAll` calls Ensure for every enabled module, used at startup).
- [ ] **Step 5: Run** `go test -race ./internal/domain/ ./internal/storage/sqlite/` → PASS. **Commit** `feat: module registry, modules table and migration 0009`.

### Task 2: Owned data items and reserved keys

**Files:**
- Modify: `internal/domain/data.go` (`DataItem.Owner string`; `DataRepo.AssetRefs`), `internal/domain/data_service.go`,
  `internal/storage/sqlite/data.go`
- Test: `internal/domain/data_service_test.go`, `internal/storage/sqlite/data_test.go`

**Interfaces:**
- Consumes: `ModuleService` (Task 1) through the new field `DataService.Modules *ModuleService` (nil: no modules).
- Produces:
  - `type AssetRef struct { Visibility Visibility; Owner string }`; `DataRepo.AssetRefs(ctx, assetID int64) ([]AssetRef, error)` (replaces `AssetIsPublic` in the repo).
  - `DataService.AssetAccess(ctx, assetID) (public, blocked bool, err error)` — public: some ref is public and its owner is not disabled; blocked: refs exist and all belong to disabled owners. `AssetIsPublic` stays as a thin wrapper.
  - `DataService.EnsureOwnedTable(ctx, owner, key, description string, cols []DataColumn, vis Visibility) (DataItem, error)` — create when missing (system actor audit `data.created` with `owner`), else set columns/description if they differ; keeps visibility.
  - `DataService.ReplaceOwnedRows(ctx, owner, key string, rows []DataRow) error` — replace all rows, refresh asset refs, touch `updated_at`; `ErrOwned` when the item's owner differs.
- [ ] **Step 1: Tests**: `Create` key `feed.anything` → validation `key: key_reserved`; owned item: `Patch` with description/value/columns → `ErrOwned`, `Patch` with visibility only → ok and audited; `UpsertRows`, `PatchRows`, `DeleteRows`, `Delete` → `ErrOwned`; `Public`, `PublicOne`, `ForUser` leave owned items of a disabled module out and include them when on; `AssetAccess` matrix (public owned on/off, residents only, unreferenced).
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement** (`Owner` scanned/stored as nullable `owner`; guard via a helper `owned(it) error`). **Step 4: Run** → PASS.
- [ ] **Step 5: httpapi asset handler** uses `AssetAccess`: public → public cache; `!blocked && ValidAssetSig` → signed; else 404. Existing public tests still pass. **Commit** `feat: module-owned data items, reserved key prefixes`.

### Task 3: Outbox images and Telegram photos

**Files:**
- Modify: `internal/domain/outbox.go` (`OutMessage.ImageAssetID int64`, `OutMessage.Image *OutImage`; `OutboxRepo.ListByOrigin`),
  `internal/domain/outbox_worker.go`, `internal/domain/notifier.go`, `internal/storage/sqlite/outbox.go`,
  `internal/messenger/telegram/telegram.go` (`API.SendPhoto`), `internal/messenger/telegram/send.go`,
  fakes in `internal/messenger/telegram/telegram_test.go` and `cmd/homelander/main_test.go`
- Test: `internal/storage/sqlite/outbox_test.go`, `internal/domain/outbox_worker_test.go`, `internal/domain/notifier_test.go`,
  `internal/messenger/telegram/send_test.go`

**Interfaces:**
- Produces:
  - `type OutImage struct { Data []byte; ContentType string }` (filled by the worker, never stored).
  - Payload JSON gains `"image": <asset id>` (omitted when 0).
  - `OutboxRepo.ListByOrigin(ctx, origin string) ([]OutboxItem, error)` ordered by id.
  - `Notifier.SendToLabels(ctx, labelIDs []int64, m OutMessage, origin string) ([]int64, error)` — destinations of all labels, each once (the row's `label_id` is the first chosen label reaching it); `sendToLabel` reuses it.
  - `telegram.API.SendPhoto(chatId int64, photo gotgbot.InputFileOrString, opts *gotgbot.SendPhotoOpts) (*gotgbot.Message, error)`.
- [ ] **Step 1: Tests**: payload round-trip keeps `ImageAssetID`; `ListByOrigin`; worker with an image row loads the asset and passes `Image` to the messenger; missing asset → `failed`, error contains `image asset`; `SendToLabels` with labels {1: chat A, 2: chat A + topic T of chat A} → 2 rows; Telegram `Send` with `Image` calls `SendPhoto` with caption = text, thread, inline keyboard, and never `SendMessage`.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement**. **Step 4: Run** `go test -race ./internal/... ./cmd/...` → PASS. **Commit** `feat: photo messages through the outbox and sendPhoto`.

### Task 4: Feed domain

**Files:**
- Create: `internal/domain/feed.go`, `internal/domain/feed_service.go`, `internal/storage/sqlite/feed.go`
- Test: `internal/domain/feed_service_test.go`, `internal/storage/sqlite/feed_test.go`

**Interfaces:**
- Consumes: `ModuleService`, `DataService.EnsureOwnedTable/ReplaceOwnedRows`, `Notifier.SendToLabels`, `OutboxRepo.ListByOrigin`.
- Produces:
  - `type PostState string` (`PostDraft`, `PostPublished`); `type FeedPost struct { ID int64; Body string; ImageAssetID *int64; State PostState; PublishedAt *time.Time; LabelIDs []int64; CreatedBy, UpdatedBy *int64; CreatedAt, UpdatedAt time.Time; DeletedAt *time.Time }`.
  - `type FeedCursor struct { At time.Time; ID int64 }` with `String()` / `ParseFeedCursor(s) (FeedCursor, error)` (`<unix nano>_<id>`).
  - `FeedRepo`: `Create(ctx, *FeedPost) error`, `Update(ctx, *FeedPost) error`, `Get(ctx, id) (FeedPost, error)` (deleted → `ErrNotFound`), `List(ctx, limit, offset int) ([]FeedPost, int, error)` (not deleted, id desc), `Published(ctx, after *FeedCursor, limit int) ([]FeedPost, error)` (published_at desc, id desc).
  - `FeedSettings struct { DefaultLabelIDs []int64 }`.
  - `FeedService{R Repos; Now Clock; Modules *ModuleService; Data *DataService; Notifier *Notifier; I18n Translator; DefaultLocale func(ctx) string; PublicURL string}`:
    `Def() ModuleDef` (code `feed`, `modules.feed.name/description`, default on, nav `{feed, /my/feed, cabinet.nav.feed, news}`, Ensure = `s.ensure`),
    `Settings(ctx) (FeedSettings, error)`, `SaveSettings(ctx, FeedSettings, a) error` (unknown label → `defaultLabelIds: invalid`),
    `Create(ctx, PostInput, a) (FeedPost, error)`, `Update(ctx, id, PostInput, a) (FeedPost, error)`, `Publish(ctx, id int64, labelIDs []int64, a) (FeedPost, error)`,
    `Delete(ctx, id, a) error`, `Get(ctx, id) (FeedPost, error)`, `List(ctx, limit, offset) ([]FeedPost, int, error)`,
    `Published(ctx, cursor string, limit int) ([]FeedPost, string, error)`, `PublishedOne(ctx, id) (FeedPost, error)`,
    `Deliveries(ctx, id) ([]OutboxItem, error)`. Every method starts with `Modules.Require(ctx, "feed")`.
  - `PostInput struct { Body string; ImageAssetID *int64; LabelIDs *[]int64 }`.
  - Constants `FeedCode = "feed"`, `FeedPostsKey = "feed.posts"`, `FeedPostsRows = 20`, `maxPostBody = 4000`, `maxPostCaption = 1000`.
- [ ] **Step 1: Repo tests**: round-trip; `List` hides deleted; `Published` paging when #2 is published before #1 (each once, order by published_at).
- [ ] **Step 2: Service tests**: body empty → `body: required`; 4001 runes → `body: too_long`; 1001 runes with image → `body: too_long`; image that is not an image asset → `imageAssetId: invalid`; create audits `feed.post_created`; publish → state, `published_at`, labels fixed, outbox rows per destination with origin `feed:<id>`, message text = body, button URL `https://example.com/my/feed/<id>`, `ImageAssetID` set for a photo; publishing again → `ErrConflict`; updating labels of a published post → `labels: published`; edit and delete of a published post queue nothing and rewrite `feed.posts` (text, date, url, image cells; deleted post gone); 25 published → 20 rows; module off → `ErrModuleOff` for every method; `Ensure` makes `feed.posts` owned, `residents`, columns as in Global Constraints; http PublicURL → no button.
- [ ] **Step 3: Run** → FAIL. **Step 4: Implement**. **Step 5: Run** → PASS. **Commit** `feat: the information feed module`.

### Task 5: HTTP API, wiring, MCP and docs

**Files:**
- Create: `internal/httpapi/modules.go`, `internal/httpapi/feed.go`
- Modify: `internal/httpapi/public.go` (API struct gains `Modules *domain.ModuleService`, `Feed *domain.FeedService`), `internal/httpapi/errors.go`
  (`ErrModuleOff` → 404 `module_disabled`, `ErrOwned` → 409 `module_owned`), `internal/httpapi/data.go` (DTO `owner` = module code or null, `ownerEnabled`),
  `internal/httpapi/admin.go` (register routes), `cmd/homelander/main.go`, `internal/mcpserver/data_tools.go` + `server.go`
  (`owner` in item output; readable `ErrOwned` text), `internal/mcpserver/guide.md`, `docs/mcp.md`, `docs/development.md`, `locales/*.json` (`feed.openInCabinet`)
- Test: `internal/httpapi/modules_test.go`, `internal/httpapi/feed_test.go`, `internal/mcpserver/server_test.go`, `cmd/homelander/feed_e2e_test.go`

**Routes (JSON shapes are the contract for Task 6):**
- `GET /api/v1/modules` (user) → `[{code, nav: [{key, path, labelKey, icon}]}]` (enabled only).
- `GET /api/v1/admin/modules` → `[{code, nameKey, descriptionKey, enabled, defaultEnabled, updatedAt, updatedBy}]`;
  `PUT /api/v1/admin/modules/{code}` `{enabled}` → one module.
- `GET|PUT /api/v1/admin/feed/settings` `{defaultLabelIds: number[]}`.
- `GET /api/v1/admin/feed/posts?page&pageSize` → `{items: AdminPost[], total}`; `POST /api/v1/admin/feed/posts` `{body, imageAssetId, labelIds}` → 201 AdminPost;
  `GET /api/v1/admin/feed/posts/{id}` → AdminPost with `deliveries`; `PATCH …/{id}` `{body, imageAssetId, labelIds?}`; `POST …/{id}/publish` `{labelIds}`; `DELETE …/{id}` → 204.
  `AdminPost = {id, body, imageAssetId, imageUrl, state, publishedAt, labelIds, createdAt, updatedAt, createdBy, updatedBy, delivery: {pending, sent, failed}, deliveryExpired, deliveries?: [{id, chatTitle, topicTitle, labelId, state, error, sentAt}]}`.
- `GET /api/v1/feed/posts?cursor` (user) → `{items: CabinetPost[], nextCursor}` (20 per page); `GET /api/v1/feed/posts/{id}`; `GET /api/v1/feed/posts/{id}/image`.
  `CabinetPost = {id, body, imageUrl, publishedAt, authorName}` (author's first name).
- [ ] **Step 1: Tests**: module off → `/modules` empty, `/feed/posts` 404 `module_disabled`, admin feed 404; on → nav entry; admin publish flow writes outbox rows; cabinet paging + `nextCursor`; image endpoint 404 for a draft; `/public/data` drops `feed.posts` (made public) when the module is off; admin data list shows `owner: "feed"`; PATCH owned item value → 409 `module_owned`; MCP `data_set_value` on `feed.posts` → error mentioning the module, creating `feed.new` → `key_reserved`.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement + wire** (`main.go`: `modules := &domain.ModuleService{...}`, `feed := &domain.FeedService{...}`, `modules.Defs = []domain.ModuleDef{feed.Def()}`, `data.Modules = modules`, `modules.EnsureAll(ctx)` after migrations, log on error).
- [ ] **Step 4: e2e** `TestFeedPostReachesSharedChatOnce` with the fake Telegram: accepted chat with two labels, publish with both → exactly one `sendMessage` to that chat; photo post → one `sendPhoto`.
- [ ] **Step 5: Docs**: guide §3 example of module data `feed.posts`, the `feed.` prefix and read-only owned items; `docs/mcp.md` same; `docs/development.md` a "Modules and the feed" note. **Run** `go test -race ./... && go vet ./... && golangci-lint run` → PASS. **Commit** `feat: modules and feed API, wiring, MCP and docs`.

### Task 6: Web — admin modules, feed admin, cabinet feed, owned data items

**Files:**
- Create: `web/src/api/modules.ts`, `web/src/api/feed.ts`, `web/src/components/LinkifiedText.tsx`,
  `web/src/pages/admin/ModulesPage.tsx`, `web/src/pages/admin/FeedAdminPage.tsx`, `web/src/pages/admin/FeedPostEditor.tsx`,
  `web/src/pages/admin/ModuleOff.tsx`, `web/src/pages/cabinet/FeedCard.tsx`, `web/src/pages/cabinet/FeedPage.tsx`, `web/src/pages/cabinet/FeedPostPage.tsx`, `web/src/pages/cabinet/PostCard.tsx`
- Modify: `web/src/App.tsx`, `web/src/pages/admin/AdminLayout.tsx`, `web/src/cabinet/sections.ts`, `web/src/cabinet/homeCards.ts`,
  `web/src/api/types.ts`, `web/src/lib/chats.ts` (move `reachesChat` here from NotificationsPage), `web/src/pages/admin/DataPage.tsx`,
  `web/src/pages/admin/DataItemPage.tsx`, `DataValueEditor.tsx`, `DataTableEditor.tsx` (`readOnly`), `web/src/pages/admin/AuditTable.tsx` (`module:`/`feed_post:` hrefs), `web/src/locales/{uk,en}.json`, `web/src/test/fixtures.ts`
- Test: `ModulesPage.test.tsx`, `FeedAdminPage.test.tsx`, `FeedPostEditor.test.tsx`, `web/src/pages/cabinet/Feed.test.tsx`, `LinkifiedText.test.tsx`, `DataItemPage.test.tsx`, `Cabinet.test.tsx`

**Interfaces:**
- Consumes: the routes and shapes of Task 5.
- [ ] **Step 1: Tests**: Modules page lists the feed card with a switch that PUTs `{enabled:false}`; feed admin off → "turned off" + switch only; posts list shows state badges and delivery counts; editor prefills labels with defaults, offers only labels that reach an accepted chat, Save draft POSTs, Publish confirms then POSTs `/publish` with the labels, published post shows per-destination status and the "Telegram copies are not changed" note, labels disabled; cabinet nav shows "Feed" only when `/modules` lists it; home shows the latest 3 posts and "All posts"; `/my/feed` loads more with `cursor`; `/my/feed/:id` shows one post; disabled module → not found; URLs in text become links; owned item page: "managed by module" alert, no delete button, value/table read-only, visibility still changeable; Data list shows module badge and "module off".
- [ ] **Step 2: Run** `cd web && npm test` → FAIL. **Step 3: Implement** (mobile-first cabinet cards, admin pages with `PageHeader`, `Card withBorder` like the rest). **Step 4: Run** `npm test && npm run lint && npm run typecheck` → PASS. **Commit** `feat(web): modules section, feed admin and the cabinet feed`.

### Task 7: Verification and PR

- [ ] `make test && make lint`; run the app per `docs/development.md` with the fake Telegram e2e as reference and check the list in the task brief (module off/on, two labels → one chat once, photo post, cabinet feed and `/my/feed/:id` on desktop and a 375 px viewport, `feed.posts` content and visibility, owned item read-only in Data and MCP, `feed.` keys refused).
- [ ] Merge `origin/feat/data-visibility` and `origin/feat/resident-cabinet`, re-run tests, push `HEAD:feat/modules-feed`, open the PR, watch CI.
