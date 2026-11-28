# Data visibility (part 1 of 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the public flag of data items with three levels (everyone / residents / admins) and
deliver residents-level data to site pages of a signed-in resident or admin.

**Architecture:** One `visibility` column on `data_items`; the domain decides who reads what; a new
session endpoint `/api/v1/me/data` serves public + residents items with images as signed, expiring
asset URLs; our `PageFrame` fetches it and posts it into the sandboxed page after the page says
`homelander:ready`. Admin panel and MCP tools get a three-option control instead of the switch.

**Tech Stack:** Go (net/http, modernc SQLite, go-sdk MCP), React + Mantine + TanStack Query, Vitest.

**Spec:** `docs/superpowers/specs/2026-11-28-resident-cabinet-and-feed-design.md` §4 (and §7, §8).

## Global Constraints

- Migration file is exactly `internal/storage/sqlite/migrations/0007_data_visibility.sql`; `is_public = 1` → `public`, `0` → `admins`.
- Values: `public`, `residents`, `admins`; default for new items `admins`.
- A reader of `residents` items is an `active` user with `is_resident = 1` or `is_admin = 1`.
- `admins` items are never returned by `/public/data*` or `/me/data`.
- `/me/data`: session required, `Cache-Control: private, no-store`, no CORS, same shape as `/public/data`.
- Signed asset URL: `/api/v1/assets/{id}?exp=<unix>&sig=<hmac>`, HMAC-SHA256 with the app secret over id+exp, about 1 hour.
- Audit: `data.visibility_set` with `{"from": ..., "to": ...}`; old `data.public_set` rows still render.
- MCP: `data_set_public` → `data_set_visibility(key, visibility)`; catalog/get report `visibility`; create tools take `visibility`.
- Docs/comments English; UI strings uk + en in parity; no real addresses or phones in fixtures.
- Stay out of Part 2's area (`/my`, PWA, login links, bot commands, LoginPage).

## Review Focus

- Signed URL with a tampered `id` but the `exp`/`sig` of another asset → 404 (the signature covers the id). Test in Task 3.
- Signed URL for an asset that exists but is not referenced by any item → still served while the signature is valid (by design); expired → 404. Test in Task 3.
- An active user who is neither resident nor admin gets only public items from `/me/data`. Test in Task 2/3.
- A `homelander:ready` from another window (not our frame) → nothing is posted. Test in Task 5.
- A frame `src` that is not one of our page URLs (`/site/...`) → no data posted even after ready. Test in Task 5.

---

### Task 1: Schema, domain model, storage

**Files:**
- Create: `internal/storage/sqlite/migrations/0007_data_visibility.sql`
- Modify: `internal/domain/data.go`, `internal/domain/audit.go`, `internal/domain/data_service.go`, `internal/storage/sqlite/data.go`
- Test: `internal/storage/sqlite/migrate_test.go`, `internal/storage/sqlite/data_test.go`, `internal/domain/data_service_test.go`

**Interfaces:**
- Produces: `type Visibility string`; `VisPublic`, `VisResidents`, `VisAdmins`; `func (v Visibility) Valid() bool`;
  `DataItem.Visibility`; `DataInput.Visibility *Visibility`; `DataRepo.List(ctx, only []Visibility)` (nil = all);
  `ActDataVisibilitySet = "data.visibility_set"` (`ActDataPublicSet` kept for old rows).

- [ ] Test `TestMigrationDataVisibility` (package sqlite): apply 0001–0006 from `Migrations` filtered into a MapFS, insert two items with `is_public` 1 and 0, migrate fully; expect `public` and `admins`, no `is_public` column, CHECK refuses `'everyone'`.
- [ ] Storage tests use `Visibility` (CRUD round trip, `List(ctx, []Visibility{VisPublic})`, `AssetIsPublic` only for `public`).
- [ ] Domain tests: create without visibility → `admins`; invalid → field `visibility: invalid`; audit sequence `data.created data.updated data.visibility_set …` with `{"from":"admins","to":"residents"}`; changing description and visibility in one patch writes `data.updated` then `data.visibility_set`.
- [ ] Migration: add column with CHECK and default `'admins'`, copy, drop `is_public`.
- [ ] Implement; run `go test ./internal/...`; commit.

### Task 2: Reading rules and signed asset URLs (domain)

**Files:** Modify `internal/domain/data_service.go`, `cmd/homelander/main.go`; Test `internal/domain/data_service_test.go`

**Interfaces:**
- Produces: `DataService.Secret []byte`; `func CanReadResidents(u User) bool`;
  `func (s *DataService) ForUser(ctx, u User) ([]DataView, error)` (public + residents for readers, else public; rate-limited per user like `Public`);
  `func (s *DataService) SignedAssetURL(id int64) string`; `func (s *DataService) ValidAssetSig(id int64, exp, sig string) bool`;
  `const AssetURLTTL = time.Hour`.
- Expiry rounds to the hour (`now.Truncate(time.Hour).Add(2*AssetURLTTL)`) so URLs stay stable for an hour and browsers can cache images; a URL is valid for 1–2 hours.

- [ ] Tests: `ForUser` for resident, admin (non-resident), active non-resident non-admin, disabled resident; `ValidAssetSig` true for a fresh URL, false for other id, changed exp, bad sig, after expiry.
- [ ] Implement; wire `Secret: cfg.AppSecret` in main; commit.

### Task 3: HTTP API

**Files:** Modify `internal/httpapi/data.go`, `internal/httpapi/public.go`; Test `internal/httpapi/data_test.go`, `apitest_test.go` (Secret)

**Interfaces:**
- Consumes: Task 2.
- Produces: admin DTO/input field `visibility`; `GET /api/v1/me/data`; asset handler accepts `exp`/`sig`.

- [ ] Tests: admin CRUD with `visibility`; `/me/data` guest 401, resident gets public + residents (not admins), non-resident user only public, `Cache-Control: private, no-store`, no `Access-Control-Allow-Origin`; residents image cell is a signed URL that serves (`private, max-age=600`), expired/tampered → 404; public image stays a plain URL.
- [ ] Implement `publicItem(v, imageURL func(id int64) string)`; commit.

### Task 4: MCP tools, guide, docs

**Files:** Modify `internal/mcpserver/data_tools.go`, `server.go` (instructions), `guide.md` §3, `docs/mcp.md`; Test `internal/mcpserver/server_test.go`

- [ ] Tests: tool list has `data_set_visibility` and not `data_set_public`; set value with `visibility: residents` reports it; `data_set_visibility` with `everyone` → validation error; guide mentions `homelander:ready`, `homelander:data`.
- [ ] Guide §3: three levels, the ready/data messages, viewer object, example that renders a public summary and a resident detail, "design for both states".
- [ ] Implement; commit.

### Task 5: Web

**Files:**
- Create: `web/src/pages/admin/VisibilityControl.tsx` (control + badge), `web/src/lib/pageUrl.ts` (`isOurPageUrl`)
- Modify: `web/src/api/types.ts`, `web/src/api/data.ts`, `web/src/api/publicData.ts` (`useMyData`, `myDataKey`), `web/src/api/public.ts` (logout drops `meData`), `web/src/components/PageFrame.tsx`, `web/src/pages/admin/DataPage.tsx`, `DataItemPage.tsx`, `DataCreateModal.tsx`, `SiteHomeTab.tsx`, `web/src/test/fixtures.ts`, `web/src/test/render.tsx` (rerender keeps providers), `web/src/locales/{uk,en}.json`
- Test: `PageFrame.test.tsx`, `DataPage.test.tsx`, `DataItemPage.test.tsx`

**Interfaces:**
- Produces: `type Visibility = 'public' | 'residents' | 'admins'`, `visibilities`; `useMyData(userId?: number)` → `Record<string, PublicItem>`;
  `isOurPageUrl(src: string): boolean` (relative `/site/…`).
- PageFrame posts `{ type: 'homelander:data', viewer, items }` to its own frame after `homelander:ready` from that frame, again on language change and on re-fetch; guest → `viewer: { signedIn: false }, items: null`.

- [ ] Tests: PageFrame posts nothing before ready, posts guest message after ready, posts items + viewer for a resident, ignores ready from another source, posts nothing for a foreign src, posts again after a language change; DataPage shows visibility badges and filters; create modal sends `visibility: 'admins'` by default; item page segmented control sends `{ visibility: 'residents' }` and shows the help text.
- [ ] Implement; `npm test && npm run lint && npm run typecheck`; commit.

### Task 6: Verify and ship

- [ ] `make test`, `make lint`.
- [ ] Run the app (docs/development.md); curl `/api/v1/me/data` as guest and resident, signed asset valid/expired/tampered.
- [ ] Push `feat/data-visibility`, open the PR against `main`, watch CI.
