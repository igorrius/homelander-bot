# Site pages (part 2 of 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An admin uploads a home page (a zip of HTML/CSS/JS/images) as a draft, previews it in the real home-page layout, publishes it, rolls back or unpublishes; visitors see the published page inside a sandboxed iframe under our own header, and the built-in page when nothing is published.

**Architecture:** Page versions and their files live in two new SQLite tables (migration `0005`). A `PageService` in `internal/domain` validates files, keeps exactly one published version, and signs short-lived preview tokens. A `/site/` handler serves page files with a page-specific CSP (`sandbox …`, `frame-ancestors 'self'`). The React home page renders our header plus `<PageFrame>` (sandboxed iframe, height and language via `postMessage`). An admin "Site page" section lists versions and uploads zips. Part 3 (MCP) will call `PageService.CreateDraft` / `PutFiles` directly, so their signatures take plain file lists.

**Tech Stack:** Go stdlib (`archive/zip`, `crypto/hmac`), SQLite, React + Mantine + TanStack Query, vitest.

**Spec:** [docs/superpowers/specs/2026-11-27-site-data-pages-and-mcp-design.md](../specs/2026-11-27-site-data-pages-and-mcp-design.md) — sections 2, 5, 7, 9 (step 2). Builds on part 1 (branch `feat/data-space`, PR #53): this branch starts from it.

## Global Constraints

- New migration file `internal/storage/sqlite/migrations/0005_pages.sql`; never edit earlier ones.
- `index.html` is required in every version.
- Allowed file extensions → content type: `.html` text/html; charset=utf-8, `.css` text/css; charset=utf-8, `.js` text/javascript; charset=utf-8, `.json` application/json, `.svg` image/svg+xml, `.png` image/png (APNG included), `.jpg`/`.jpeg` image/jpeg, `.webp` image/webp, `.gif` image/gif, `.woff2` font/woff2. Anything else → `files: type`.
- File path: `^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9][A-Za-z0-9._-]*)*$`, ≤ 200 bytes, no segment `..`, lower-case extension; else `files: invalid`.
- Limits: ≤ 5 MiB per file, ≤ 20 MiB per version, ≤ 200 files; else `files: too_large` / `files: too_many`.
- Exactly one version is `published`, or none. Statuses: `draft` (editable), `published`, `archived` (both read-only). Publishing archives the previous published version. Published versions cannot be deleted.
- Page file responses carry this CSP, replacing the global one:
  `sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation; default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'`
  plus `X-Content-Type-Options: nosniff`. Published files: `Cache-Control: no-cache` + `ETag` (file sha256). Preview files: `Cache-Control: private, no-store`.
- The iframe: `sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation"` — never `allow-same-origin`.
- Preview token: `<versionID>.<unix expiry>.<base64url HMAC-SHA256(secret, "page-preview:"+versionID+"."+expiry)>`, TTL **30 minutes**, secret = `cfg.AppSecret`. Drafts are reachable only through a valid token for that version.
- Audit actions, target `page:<id>`: `page.draft_created`, `page.draft_updated`, `page.published`, `page.unpublished`, `page.deleted`.
- `postMessage` protocol: page → shell `{ type: "homelander:height", height: number }` (accepted only from the frame's own `contentWindow`, clamped to 100…20000 px); shell → page `{ type: "homelander:lang", lang }`; the frame URL carries `?lang=<uk|en>`.
- All new UI text through i18n (uk default, en). No real addresses or phones in fixtures.

## Review Focus

1. A draft URL without a token, with an expired token, or with a token for another version → 404 (test in Task 3).
2. Paths `../x.html`, `/abs.html`, `a//b.css`, `a\b.css`, `x.HTML`, `x.exe`, `.hidden` → rejected with the field reason above (test in Task 2).
3. A zip whose entries all sit in one folder (`site/index.html`, `site/app.js`) is stored without that folder; a zip whose uncompressed total passes 20 MiB is rejected without reading it all into memory (test in Task 4).
4. Publishing version B while A is published leaves exactly one published (B) and A archived; unpublishing makes `GET /api/v1/site` return `pageUrl: null` and the home page show the built-in page (tests in Tasks 2, 4, 6).
5. A `homelander:height` message from any window other than the frame is ignored; huge heights are clamped (test in Task 5).

---

### Task 1: Schema, types, repository

**Files:**
- Create: `internal/storage/sqlite/migrations/0005_pages.sql`, `internal/domain/page.go` (types + `PageRepo`), `internal/storage/sqlite/pages.go`, `internal/storage/sqlite/pages_test.go`
- Modify: `internal/domain/users_service.go` (`Repos.Pages`), `internal/storage/sqlite/repos.go`

**Interfaces:**
- Produces:
  ```go
  type PageStatus string // "draft" | "published" | "archived"
  type PageFile struct { Path, ContentType string; Data []byte; SHA256 string; Size int }
  type PageVersion struct {
      ID int64; Status PageStatus; Note string
      CreatedBy *int64; CreatedAt, UpdatedAt time.Time
      PublishedBy *int64; PublishedAt *time.Time
      Files []PageFile // metadata only (no Data) unless loaded with File/Files
      TotalSize int
  }
  type PageRepo interface {
      Create(ctx context.Context, v *PageVersion) error
      Update(ctx context.Context, v *PageVersion) error            // status, note, published_*, updated_at
      Delete(ctx context.Context, id int64) error                  // cascades files
      Get(ctx context.Context, id int64) (PageVersion, error)      // Files = metadata, sorted by path
      List(ctx context.Context) ([]PageVersion, error)             // newest first, Files = metadata
      Published(ctx context.Context) (PageVersion, error)          // ErrNotFound when none
      PutFiles(ctx context.Context, id int64, files []PageFile) error   // insert or replace by path
      DeleteFiles(ctx context.Context, id int64, paths []string) error
      File(ctx context.Context, id int64, path string) (PageFile, error) // with Data
      Files(ctx context.Context, id int64) ([]PageFile, error)           // with Data (copy-to-draft)
  }
  ```
- Schema: `page_versions(id, status CHECK IN (...), note TEXT NOT NULL DEFAULT '', created_by/published_by REFERENCES users ON DELETE SET NULL, created_at, updated_at, published_at)` with `CREATE UNIQUE INDEX page_versions_one_published ON page_versions(status) WHERE status = 'published'`; `page_files(version_id REFERENCES page_versions ON DELETE CASCADE, path, content_type, data BLOB, sha256, size, PRIMARY KEY(version_id, path))`.

- [ ] **Step 1: Write failing tests** `TestPageVersionCRUD` (create, put two files, `Get` returns file metadata sorted without `Data`, `File` returns bytes, replace one file by path, `DeleteFiles`, delete version → `ErrNotFound`), `TestOnePublished` (two versions set `published` → second `Update` returns `ErrConflict`; `Published` returns the one).
- [ ] **Step 2: Run** `go test ./internal/storage/sqlite -run Page` — Expected: FAIL (undefined).
- [ ] **Step 3: Implement** following `data.go` / `labels.go` patterns.
- [ ] **Step 4: Run** `go test ./internal/storage/...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(pages): schema and repository for page versions`.

### Task 2: PageService — validation, lifecycle, preview tokens

**Files:**
- Create: `internal/domain/page_service.go`, `internal/domain/page_service_test.go`
- Modify: `internal/domain/audit.go` (actions, `PageTarget(id int64) string`), `internal/domain/testenv_test.go` (`e.pages`)

**Interfaces:**
- Consumes: Task 1.
- Produces:
  ```go
  type PageService struct { R Repos; Now Clock; Secret []byte }
  type PageFileInput struct { Path string; Data []byte }
  func (s *PageService) List(ctx context.Context) ([]PageVersion, error)
  func (s *PageService) Get(ctx context.Context, id int64) (PageVersion, error)
  func (s *PageService) CreateDraft(ctx context.Context, files []PageFileInput, note string, a Actor) (PageVersion, error)
  func (s *PageService) PutFiles(ctx context.Context, id int64, put []PageFileInput, del []string, a Actor) (PageVersion, error) // drafts only, else ErrConflict
  func (s *PageService) CopyToDraft(ctx context.Context, id int64, a Actor) (PageVersion, error)
  func (s *PageService) Publish(ctx context.Context, id int64, a Actor) error   // draft or archived; idempotent on the published one
  func (s *PageService) Unpublish(ctx context.Context, a Actor) error           // no-op when none
  func (s *PageService) Delete(ctx context.Context, id int64, a Actor) error    // published → ErrConflict
  func (s *PageService) Published(ctx context.Context) (PageVersion, bool, error)
  func (s *PageService) PublishedFile(ctx context.Context, path string) (PageFile, error)
  func (s *PageService) PreviewToken(id int64) string
  func (s *PageService) PreviewFile(ctx context.Context, token, path string) (PageFile, error) // bad/expired/foreign token → ErrNotFound
  ```
- Validation per Global Constraints; the version after `PutFiles` must still contain `index.html` (`files: index_required`). Note ≤ 200 runes. Content type comes from the extension, never from the bytes.

- [ ] **Step 1: Write failing tests**:
  - `TestPagePaths` — table of the Review Focus 2 paths → `files: invalid` (or `type` for `x.exe`); `css/app.css`, `img/a-1.png` accepted.
  - `TestPageLimits` — 5 MiB+1 file → `files: too_large`; 201 files → `files: too_many`; 5 × 4.5 MiB → `files: too_large`; no `index.html` → `files: index_required`; `PutFiles` deleting `index.html` → same.
  - `TestPageLifecycle` — create A, B; publish A; publish B → A `archived`, B `published`; `PutFiles` on B → `ErrConflict`; `Delete` B → `ErrConflict`; `CopyToDraft` A → new draft with A's files; `Unpublish` → `Published` false; audit actions in order `page.draft_created ×2, page.published ×2, page.draft_created, page.unpublished`.
  - `TestPreviewToken` — token for draft A reads A's `index.html`; the token with its version id changed to B's (signature kept) → `ErrNotFound`; token after 31 minutes (`e.clock.Advance`) → `ErrNotFound`; tampered signature → `ErrNotFound`; a token for a deleted version → `ErrNotFound`.
- Note: the token is minted only by the admin route `POST /admin/pages/{id}/preview` and lives 30 minutes; it is a bearer link, not bound to the session (spec 6.3: the agent never receives it, only the `/admin/pages/{id}` link).
- [ ] **Step 2: Run** `go test ./internal/domain -run 'Page|Preview'` — Expected: FAIL.
- [ ] **Step 3: Implement.** Publish/Unpublish/Delete/PutFiles run in `R.Tx.InTx` with their audit entry. HMAC compare with `hmac.Equal`.
- [ ] **Step 4: Run** `go test -race ./internal/domain` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(pages): page service with validation, publishing and preview tokens`.

### Task 3: Serving page files under `/site/`

**Files:**
- Create: `internal/httpapi/pages.go` (site handler + admin handlers of Task 4), `internal/httpapi/pages_test.go`
- Modify: `internal/httpapi/server.go` (`func (s *Server) MountPrefix(prefix string, h http.Handler)`; root switch checks prefixes before the SPA), `internal/httpapi/public.go` (`API.Pages *domain.PageService`; `GET /api/v1/site` gains `"pageUrl": "/site/index.html" | null`), `internal/httpapi/admin.go` (`writeSite` same field), `cmd/homelander/main.go` (wire `PageService{Secret: cfg.AppSecret}`, `server.MountPrefix("/site/", httpapi.SiteHandler(api))`), `internal/httpapi/apitest_test.go`

**Interfaces:**
- Produces: `func SiteHandler(api API) http.Handler` serving `GET|HEAD /site/{path...}` (published) and `/site/preview/{token}/{path...}` (draft); `/site/` alone → `index.html`. Missing file or no published version → 404 plain text.

- [ ] **Step 1: Write failing tests** `TestSiteServesPublished` (body, content type by extension, exact CSP string from Global Constraints, `nosniff`, `ETag`, `If-None-Match` → 304, global `frame-ancestors 'none'` absent), `TestSiteNothingPublished` (404; `GET /api/v1/site` → `pageUrl` null), `TestSitePreview` (valid token → 200 with `Cache-Control: private, no-store`; no token path `/site/preview/` → 404; expired / other version's token → 404), `TestSitePageURL` (after publish `GET /api/v1/site` → `pageUrl == "/site/index.html"`).
- [ ] **Step 2: Run** `go test ./internal/httpapi -run Site` — Expected: FAIL.
- [ ] **Step 3: Implement.** Set headers after `securityHeaders` ran (the handler overwrites `Content-Security-Policy`).
- [ ] **Step 4: Run** `go test -race ./internal/httpapi` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(pages): serve published and preview page files with a sandbox CSP`.

### Task 4: Admin pages API and zip upload

**Files:**
- Modify: `internal/httpapi/pages.go`, `internal/httpapi/admin.go` (`registerPages(h, routes)`), `internal/httpapi/pages_test.go`
- Create: `internal/httpapi/zip.go` (`func readZip(r io.ReaderAt, size int64) ([]domain.PageFileInput, error)`), `internal/httpapi/zip_test.go`

**Interfaces:**
- Produces (all `RequireAdmin`):
  - `GET /api/v1/admin/pages` → `[versionDTO]`: `{id, status, note, files: [{path, contentType, size}], totalSize, createdAt, createdBy: {id,name}|null, publishedAt, publishedBy}`
  - `GET /api/v1/admin/pages/{id}` → `versionDTO`
  - `POST /api/v1/admin/pages` multipart `zip` (+ optional `note`) → 201 `versionDTO` (draft). Body cap 25 MiB.
  - `POST /api/v1/admin/pages/{id}/copy` → 201 `versionDTO`
  - `POST /api/v1/admin/pages/{id}/publish` → 204; `POST /api/v1/admin/pages/unpublish` → 204
  - `DELETE /api/v1/admin/pages/{id}` → 204 (published → 409 `conflict`)
  - `POST /api/v1/admin/pages/{id}/preview` → `{ "url": "/site/preview/<token>/index.html" }`
- `readZip`: skips directories and `__MACOSX/`, `.DS_Store`; strips one common top-level folder; rejects when the sum of `UncompressedSize64` > 20 MiB or > 200 entries before reading data (`files: too_large` / `too_many`); reads each entry through `io.LimitReader(5 MiB+1)`. Not a zip → `zip: invalid`.

- [ ] **Step 1: Write failing tests** `TestReadZip` (common folder stripped; junk entries skipped; declared size > 20 MiB rejected; non-zip → `zip: invalid`), `TestAdminPagesFlow` (upload → list → preview url opens 200 → publish → `/site/index.html` 200 → upload second, publish → first archived → copy first → unpublish → `/site/index.html` 404 → delete archived 204, delete published 409), `TestAdminPagesRequireAdmin` (resident 403, guest 401).
- [ ] **Step 2: Run** `go test ./internal/httpapi -run 'Zip|AdminPages'` — Expected: FAIL.
- [ ] **Step 3: Implement**, reusing `readUpload`-style multipart handling with the 25 MiB cap.
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(pages): admin API for page versions with zip upload`.

### Task 5: Web — PageFrame and the home page

**Files:**
- Create: `web/src/components/PageFrame.tsx`, `web/src/components/PageFrame.test.tsx`
- Modify: `web/src/api/types.ts` (`Site.pageUrl: string | null`), `web/src/pages/HomePage.tsx`, `web/src/pages/HomePage.test.tsx`, `web/src/test/fixtures.ts` (site fixture `pageUrl: null`)

**Interfaces:**
- Produces: `export function PageFrame({ src, lang, title }: { src: string; lang: string; title: string })` — renders `<iframe src={src + '?lang=' + lang} sandbox="…" title={title} style={{ width: '100%', border: 0, height }}>`; listens to `message`; posts `{type:'homelander:lang', lang}` to `contentWindow` when `lang` changes. Initial height 600.

- [ ] **Step 1: Write failing tests** in `PageFrame.test.tsx`: sandbox attribute equals the Global Constraints string exactly and lacks `allow-same-origin`; `src` ends with `?lang=uk`; a `MessageEvent` with `source` = the iframe's `contentWindow` and height 900 sets `style.height` to `900px`; the same message from `window` (other source) changes nothing; height 999999 → `20000px`; switching language posts `homelander:lang` (spy on `contentWindow.postMessage`). In `HomePage.test.tsx`: with `pageUrl: '/site/index.html'` the home page renders the frame (by `title`) and still the login button; with `pageUrl: null` the built-in page shows as before.
- [ ] **Step 2: Run** `cd web && npx vitest run PageFrame HomePage` — Expected: FAIL.
- [ ] **Step 3: Implement.** `HomePage`: when `site.data?.pageUrl` → `PageShell` header (name + `Account`) and `<PageFrame>` as the body; otherwise the current content.
- [ ] **Step 4: Run** `cd web && npm test && npm run lint && npm run typecheck` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): home page shows the published page in a sandboxed frame`.

### Task 6: Web — admin "Site page" section with preview

**Files:**
- Create: `web/src/api/pages.ts`, `web/src/pages/admin/PagesPage.tsx`, `web/src/pages/admin/PagesPage.test.tsx`, `web/src/pages/admin/PagePreviewPage.tsx`
- Modify: `web/src/api/types.ts` (`PageVersion`, `PageStatus`), `web/src/App.tsx` (routes `pages`, `pages/:id`), `web/src/pages/admin/AdminLayout.tsx` (nav item after Site, icon `IconLayout`, `admin.nav.pages`), locales uk/en (`admin.pages.*`, `audit.action.page.*`, `validation.index_required`, `validation.too_large`, `validation.too_many`)

**Interfaces:**
- Consumes: Task 4 routes; Task 5 `PageFrame`.
- Produces: hooks `usePageVersions()`, `usePageVersion(id)`, `useUploadPage()`, `useCopyPage()`, `usePublishPage()`, `useUnpublishPage()`, `useDeletePage()`, `usePagePreview(id)`; query key `['admin','pages']`.
- `/admin/pages/:id` (`PagePreviewPage`): status, note, file list, **Publish** (confirm), **Copy to draft**, **Delete**, and below them the preview: our home-page header look + `PageFrame` with the preview URL. This is also the link MCP (part 3) returns.

- [ ] **Step 1: Write failing tests** in `PagesPage.test.tsx`: list shows status badges (`published`/`draft`/`archived`) and author; uploading a `.zip` via the file input posts `POST /admin/pages` (FormData `zip`) and opens `/admin/pages/<new id>`; 422 `files: index_required` shows `uk.validation.index_required`; "Unpublish" (confirm) posts `POST /admin/pages/unpublish`; on `/admin/pages/3` Publish (confirm) posts `POST /admin/pages/3/publish`, the frame `src` starts with the preview URL returned by `POST /admin/pages/3/preview`; Delete is absent for the published version.
- [ ] **Step 2: Run** `cd web && npx vitest run PagesPage` — Expected: FAIL.
- [ ] **Step 3: Implement** in the style of `DataPage` / `DataItemPage`.
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): site page versions, preview and publishing in the admin panel`.
