# Site redesign and multi-page sites Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A built-in home page that looks finished (isometric quarter C1, one-line header, data cards), sites with several pages at their own addresses, one admin "Site" section with tabs, and a draft preview that switches between pages.

**Architecture:** Pages stay files of one version; a pure function in `internal/domain` derives the page list from a version's root `*.html` files. The site API exposes the published pages; the SPA gets a catch-all `/:page` route rendering the shell + frame. Preview HTML gets a tiny injected navigation script. The built-in page is new React components fed by houses, site settings (`homeItems`) and the public data endpoint.

**Tech Stack:** Go, SQLite, React + Mantine + TanStack Query, SVG, vitest.

**Spec:** [docs/superpowers/specs/2026-11-27-site-redesign-and-multipage-design.md](../specs/2026-11-27-site-redesign-and-multipage-design.md)

## Global Constraints

- Page name regex `^[a-z0-9][a-z0-9-]{0,62}$`; `index` = `/`. Bad root `.html` name on new files → `files: invalid_page_name`; reserved → `files: reserved_page_name`.
- Reserved: `admin`, `login`, `register`, `oauth`, `site`, `api`, `mcp`, `healthz`, `assets`, `telegram`, `favicon`, `robots`, `sitemap`, and any name starting with `.`.
- In any version, a root `.html` breaking the rules is not a page (no route); restore and copy-to-draft are not blocked.
- Whole site is published at once; page files keep being served under `/site/…` with the part-2 sandbox CSP unchanged.
- Preview-only injected script posts `{ type: "homelander:navigate", path }`; published pages get no injected script. `homelander:height` / `homelander:lang` unchanged.
- `url` and `phone` are value types as well as column types.
- Site picture removed: routes `PUT/DELETE /api/v1/admin/site/picture`, `SetPicture`/`RemovePicture`, `pictureUrl`, the picture clause of `AssetIsPublic`, picture UI. Stored `pictureAssetId` ignored, no migration.
- Site settings gain `homeItems: [{ key: string, title: LocalizedText }]` (ordered) — JSON in `settings`, no migration. *Planner's ruling:* the spec's card title "item description or key" cannot come from the public endpoint (part 1 keeps descriptions private), so the admin sets each card's title in the picker; an empty title falls back to the prettified key (`house.address` → "House address").
- i18n uk default + en; no real addresses/phones in fixtures; existing tests stay green.

## Review Focus

1. A long community name (80+ chars) and a signed-in admin with 3 apartments: header stays one line, name truncated with ellipsis, account in a menu (test in Task 6).
2. 0, 1, 24 houses and houses without `floors`: the quarter renders without overlap or NaN in SVG attributes (test in Task 6).
3. `/contacts` when nothing is published, when the published version has no `contacts.html`, and `/Contacts` → not-found page, never a blank frame (test in Task 3).
4. In preview, a link to a page that is not in the draft (`/rules` missing) or to `/admin` is not intercepted — it behaves as a normal link (test in Task 2).
5. `homeItems` listing a key that became private or was deleted → that card is silently absent, others render (test in Task 7).

---

### Task 1: Page names in the domain

**Files:**
- Modify: `internal/domain/page.go` (types), `internal/domain/page_service.go` (`toFiles` checks), `internal/domain/page_service_test.go`

**Interfaces:**
- Produces:
  ```go
  type SitePage struct { Name string; Path string; File string } // index → Path "/"
  var ReservedPageNames = []string{"admin","login","register","oauth","site","api","mcp","healthz","assets","telegram","favicon","robots","sitemap"}
  func PagesOf(files []PageFile) []SitePage // sorted: "/" first, then by name; skips names breaking the rules
  func checkPageName(file string) (reason string) // "" | "invalid_page_name" | "reserved_page_name"; only for root *.html
  ```
- `CreateDraft` / `PutFiles` (and so zip upload and MCP) reject bad names with `files: <reason>`; `Publish`/`CopyToDraft` do not check.

- [ ] **Step 1: Failing tests** `TestPagesOf` (files `index.html`, `contacts.html`, `Rules.html`, `admin.html`, `.x.html`, `css/a.html`, `app.js` → `[{index,/},{contacts,/contacts}]`), `TestPageNameRules` (`CreateDraft` with `admin.html` → `files: reserved_page_name`; `Rules.html` → `files: invalid_page_name`; `css/Nested.html` accepted), `TestRestoreOldNames` (a version stored directly through the repo with `Admin.html` publishes and copies fine).
- [ ] **Step 2: Run** `go test ./internal/domain -run 'PagesOf|PageName|RestoreOld'` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/domain` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(pages): page names from root html files, reserved names`.

### Task 2: Pages in the API and preview navigation script

**Files:**
- Modify: `internal/httpapi/pages.go` (DTO `pages: [{name, path}]`; preview HTML injection), `internal/httpapi/admin.go` (`writeSite`: `pages: []string` of the published version), `internal/httpapi/pages_test.go`
- Create: `internal/httpapi/preview_nav.js` (embedded) 

**Interfaces:**
- `GET /api/v1/site` → adds `"pages": ["/", "/contacts"]` (`[]` when nothing published).
- Admin version DTO adds `pages: [{name, path}]`.
- Preview `.html` responses: the server inserts `<script>…</script>` (from `preview_nav.js`) right before `</body>` (or appends at the end when absent). The script: on `click` (capture phase) of an `<a>` whose resolved URL is same-origin and whose path is in the draft's page list (embedded as JSON in the script), `preventDefault()` and `parent.postMessage({type:'homelander:navigate', path}, '*')`. Published responses are byte-identical to the stored file.

- [ ] **Step 1: Failing tests** `TestSitePagesList`, `TestVersionDTOPages`, `TestPreviewInjectsNav` (preview `index.html` contains the script, the text `homelander:navigate` and the page list `["/","/contacts"]`; preview `app.css` unchanged; published `index.html` byte-identical to stored, ETag unchanged). The parent-side filtering of Review Focus 4 is tested in Task 5.
- [ ] **Step 2: Run** `go test ./internal/httpapi -run 'SitePages|VersionDTO|PreviewInjects'` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(pages): list pages in the API; preview links stay in the preview`.

### Task 3: Public routes for pages and the shared site header

**Files:**
- Create: `web/src/components/SiteHeader.tsx` (+ test), `web/src/pages/SitePageRoute.tsx` (+ test), `web/src/lib/reservedPages.ts`
- Modify: `web/src/App.tsx` (route `/:page` after all named routes), `web/src/pages/HomePage.tsx` (use `SiteHeader`), `web/src/api/types.ts` (`Site.pages: string[]`, drop `pictureUrl` in Task 8), locales

**Interfaces:**
- `SiteHeader({ title }: { title: string })`: one line; left `title` (one line, ellipsis, full text in `title` attribute); right: guest → "Log in" button and "Register" link; signed-in → `Menu` trigger (avatar initials + first name) with items: apartments (text), "Admin panel" (admins), "Log out"; then `LanguageSwitcher`. Mobile: name and menu stay on one line.
- `SitePageRoute`: reads `:page`; if `"/" + page` ∈ `site.pages` → `SiteHeader` + `PageFrame src=/site/<page>.html`; else `NotFoundPage`. `reservedPages.ts` exports the same list as the Go one; a vitest checks every top-level `<Route path>` in `App.tsx` is reserved.

- [ ] **Step 1: Failing tests**: header with an 80-char name and an admin with 3 apartments → name has `text-overflow` style and the full name in `title`, menu items appear after clicking the trigger (Review Focus 1); `/contacts` with `pages: ['/','/contacts']` renders the frame `src` `/site/contacts.html?lang=uk`; `/contacts` with `pages: ['/']`, with `pages: []`, and `/Contacts` → not-found (Review Focus 3); reserved list covers every top-level route.
- [ ] **Step 2: Run** `cd web && npx vitest run SiteHeader SitePageRoute reserved` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npm test && npm run lint && npm run typecheck` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): pages at their own addresses and a one-line site header`.

### Task 4: `url` and `phone` value types and `homeItems`

**Files:**
- Modify: `internal/domain/data_service.go` (`valueTypes` += `DataURL`, `DataPhone`), `internal/domain/site.go` (`SiteSettings.HomeItems []string`; drop `PictureAssetID` in Task 8), `internal/domain/catalog_service.go` (`SaveSite` validates `homeItems`: ≤ 20 entries, each `key` a valid data key, title ≤ 100 runes per locale, duplicate keys removed keeping the first; unknown keys allowed), `internal/storage/sqlite/site.go`, `internal/httpapi/admin.go` (`saveSite` / `writeSite` with `homeItems`), `internal/mcpserver/data_tools.go` (description of `value_type` lists url, phone), `web/src/api/types.ts` (`valueTypes` += url, phone; `Site.homeItems`), `web/src/pages/admin/DataValueEditor.tsx` (TextInput for url/phone), tests.

- [ ] **Step 1: Failing tests**: `TestDataValueURLPhone` (value `url` `"https://x.test"` ok, `"ftp://x"` → `value: invalid`; `phone` `"+380 44 000 00 00"` ok, `"call me"` invalid); `TestSiteHomeItems` (save keys `contacts, house.address, contacts` → stored `contacts, house.address` with their titles; key `Bad Key` → `homeItems: invalid`; 21 items → `homeItems: too_many`); web: value editor shows a text input for a `phone` value and PATCHes it.
- [ ] **Step 2: Run** `go test ./internal/... -run 'URLPhone|HomeItems' && cd web && npx vitest run DataItemPage` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `make test` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(data): url and phone values; site homeItems`.

### Task 5: Preview with page switching

**Files:**
- Modify: `web/src/pages/admin/PagePreviewPage.tsx` (+ test), `web/src/components/PageFrame.tsx` (optional `onNavigate(path)` prop; accepts `homelander:navigate` only from its own frame and only for paths in a `pages` prop)

**Interfaces:**
- `PageFrame({ src, lang, title, pages?, onNavigate? })`.
- Preview page: `SegmentedControl` (≤ 6 pages) or `Select` (more) over `version.pages`; selecting or a `homelander:navigate` message switches `src` to `/site/preview/<token>/<file>`.

- [ ] **Step 1: Failing tests**: switcher lists `/` and `/contacts` and changes the frame src; a `homelander:navigate` to `/contacts` from the frame switches; the same message from `window` or with `/admin` or `/rules` (not in the draft) is ignored (Review Focus 4).
- [ ] **Step 2: Run** `cd web && npx vitest run PagePreview PageFrame` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npm test` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): switch between pages in the draft preview`.

### Task 6: Built-in home page — the quarter

**Files:**
- Create: `web/src/components/Quarter.tsx` (+ test) replacing `HousesIllustration.tsx` (delete it and its test), `web/src/lib/quarterLayout.ts` (+ test)
- Modify: `web/src/pages/HomePage.tsx` (hero: full community name as `Title`, welcome text; then `Quarter`), `internal/httpapi/public.go` (`register/options` houses gain `floors`), `web/src/api/types.ts`

**Interfaces:**
- `quarterLayout(n: number): { cols: number; rows: number; cells: { col: number; row: number; house: number | null }[] }` with `cols = max(2, ceil(sqrt(n*1.8)))` for n ≥ 1, `rows = max(1, ceil(n/cols))`; n = 0 → one empty yard with trees.
- `Quarter({ houses }: { houses: { id: number; name: string; floors: number | null }[] })`: isometric SVG — green yard, paths, trees/bushes in empty cells and gaps, front street with sidewalk, markings and lamps; block height from `floors` clamped to 3…25 (default 9); palette from Mantine theme so it works in dark mode; label = house name truncated to 18 chars with `<title>`; `role="img"` and `aria-label` listing names; `width="100%"`, `max-height` 420px desktop / auto on mobile. Mockup reference: `.superpowers/brainstorm/*/content/home-iso-v3.html` (sketch only).

- [ ] **Step 1: Failing tests**: `quarterLayout` for 0, 1, 2, 12, 24 (counts, every house placed once); `Quarter` renders 24 house groups with no `NaN` in any attribute, a house with `floors: null` renders, aria-label lists names (Review Focus 2).
- [ ] **Step 2: Run** `cd web && npx vitest run Quarter quarterLayout HomePage` — Expected: FAIL.
- [ ] **Step 3: Implement**; check by eye in the browser with 2, 12 and 24 houses in light and dark theme and at 375 px width.
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): isometric quarter on the built-in home page`.

### Task 7: Built-in home page — data cards

**Files:**
- Create: `web/src/components/HomeCards.tsx` (+ test), `web/src/api/publicData.ts` (`usePublicData()` → `GET /api/v1/public/data`)
- Modify: `web/src/pages/HomePage.tsx`

**Interfaces:**
- `HomeCards({ items, data }: { items: { key: string; title: LocalizedText }[]; data: Record<string, PublicItem> })`: in `items` order, skip keys missing from `data`; card title = `title` in the current language, else the prettified key. Phone → `tel:` link, url → link (`rel="noopener"`, new tab), image → `Image`, localized_text → current language, bool → Yes/No, table → list of rows: cells joined with " · ", phone/url cells as links.

- [ ] **Step 1: Failing tests**: two keys render in order; a missing key is skipped (Review Focus 5); phone renders `href="tel:+380440000000"` (spaces stripped); table rows render with links.
- [ ] **Step 2: Run** `cd web && npx vitest run HomeCards HomePage` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npm test` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): data cards under the quarter`.

### Task 8: Remove the site picture

**Files:** `internal/domain/catalog_service.go`, `internal/domain/site.go`, `internal/domain/audit.go` (keep action constants for old entries), `internal/httpapi/admin.go`, `internal/httpapi/admin_test.go`, `internal/storage/sqlite/data.go` (`AssetIsPublic` drops the site clause), `internal/storage/sqlite/site.go` (ignore stored `pictureAssetId`), `web/src/pages/admin/SitePage.tsx` (picture block), `web/src/api/admin.ts`, `web/src/api/types.ts`, locales, tests.

- [ ] **Step 1: Failing tests**: `PUT /api/v1/admin/site/picture` → 404; `GET /api/v1/site` has no `pictureUrl`; settings JSON with an old `pictureAssetId` loads fine; an asset that was the site picture is no longer public.
- [ ] **Step 2: Run** `go test ./internal/... -run 'Site|Asset'` — Expected: FAIL.
- [ ] **Step 3: Implement**; `git grep -n -i "picture"` must show only audit labels for old entries and locale keys still used by them.
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `refactor(site): drop the uploaded site picture`.

### Task 9: Admin "Site" section with tabs

**Files:**
- Create: `web/src/pages/admin/SiteSection.tsx` (+ test), `web/src/pages/admin/SitePagesTab.tsx`, `web/src/pages/admin/SiteHomeTab.tsx`, `web/src/pages/admin/VersionCard.tsx`
- Modify: `web/src/App.tsx` (`/admin/site` → `SiteSection`; `/admin/pages` → `<Navigate to="/admin/site?tab=pages">`; `/admin/pages/:id` unchanged), `web/src/pages/admin/AdminLayout.tsx` (one "Site" nav item; remove "Site page"), delete `SitePage.tsx`/`PagesPage.tsx` and move their tests' still-valid cases, locales

**Interfaces:**
- Tabs `?tab=pages` (default) / `?tab=home`.
- Pages tab: green "Live now" `VersionCard` (note, author + MCP badge, published at, page chips linking to the real site, "Open site ↗", "Return to the built-in page" with confirm) or the empty state "Nothing published — residents see the built-in page"; yellow "Drafts" (each card: note, author, MCP badge, created, page count, primary "Preview and publish" → `/admin/pages/:id`, Delete with confirm) or "No drafts. Ask your AI agent or upload a zip"; "Archive" `Accordion` collapsed with "Restore" (publish, confirm) and Delete; "Upload zip" in the header.
- Home tab: community name, welcome text, default language (existing form), "Show under the quarter" — add **public** items from `GET /admin/data` (a select), each row with a card title (`LocalizedTextInputs`, optional), up/down and remove; hint when nothing is public; Save → `PUT /admin/site` with `homeItems`; a scaled mini preview (`HomePage` body in a fixed-width box).

- [ ] **Step 1: Failing tests**: nav shows one "Site" item; `/admin/pages` redirects; live block shows chips `/` and `/contacts` with links; drafts show "Preview and publish" linking to `/admin/pages/8`; archive restore posts publish after confirm; return-to-built-in posts unpublish; home tab saves `homeItems` (keys and titles) in the chosen order and offers only public items.
- [ ] **Step 2: Run** `cd web && npx vitest run SiteSection AdminLayout` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): one Site section with Pages and Built-in page tabs`.

### Task 10: Agent side and docs

**Files:** `internal/mcpserver/page_tools.go` (`pages: [{name, path}]` in `page_list_versions` / `page_get_version` / create/update results), `internal/mcpserver/guide.md` ("Several pages": naming, reserved names, `target="_top"` links, shared CSS/JS, editing flow list → get → copy_to_draft → update_draft → give the admin the link; url/phone values), `internal/mcpserver/server_test.go`, `docs/mcp.md`.

- [ ] **Step 1: Failing tests**: `page_create_draft` with `index.html` + `contacts.html` returns `pages` with `/contacts`; `admin.html` → `IsError` containing `reserved_page_name (admin.html)`; guide mentions `homelander:navigate` is not needed by agents and contains "Several pages".
- [ ] **Step 2: Run** `go test ./internal/mcpserver` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): pages in tool results; guide for multi-page sites`.
