# Site redesign: built-in home page, multi-page sites, one "Site" section — design

- **Date:** 2026-11-27
- **Status:** Approved by the owner (2026-11-27)
- **Owner:** Igor Artasevych
- **Builds on:** [site data, pages and MCP design](2026-11-27-site-data-pages-and-mcp-design.md) (all three parts merged: #53, #56, #57)

## 1. Intent

The site works but does not look or feel finished:

- The **built-in home page** (shown when no page is published) is a flat row of cartoon houses on a
  bare background. A long community name wraps over two lines and pushes the signed-in account block
  into a column (name, apartment, admin link, logout stacked). With many houses the picture shrinks to
  nothing. The owner calls it "scary"; it must look like a million dollars and stay tidy with 2, 12 or
  24 houses.
- The admin panel has **two sections** for one thing — "Site" (name, welcome text, picture, language)
  and "Site page" (versions) — and the owner "did not understand what is what". The versions list is a
  bare table with no sense of what is live, what waits for review, and what is history.
- A published site is **one page**. The owner wants several pages with their own addresses
  (`/contacts`, `/rules`, …) that link to each other, created by the AI agent through MCP and, as now,
  previewed and published by an admin.

The owner will try building a custom home page through MCP right after this lands; until then the
built-in page is what residents see.

## 2. Goals, non-goals, anti-goals

### Goals

1. **Built-in home page, direction C1 "isometric quarter"** (chosen from mockups):
   - one-line header: short community name left; right a compact account control (guest: "Log in";
     signed-in: avatar + name opening a menu with apartments, "Admin panel" for admins, "Log out") and
     the UK/EN switch;
   - hero: the community name as a large heading (wrapping freely without breaking the header) and the
     welcome text;
   - an isometric quarter drawn from the houses: one block per non-archived house with its name and
     its floors (`House.Floors`, a default when unset), a shared green yard with trees, bushes and
     paths between houses, and a street with sidewalk, markings and lamps along the front edge;
     houses spaced so they never overlap; the grid adapts to the count (2, 12 and 24 houses must look
     intentional); light and dark theme; scales on phones;
   - **cards from the data space** under the quarter: the admin picks public items and their order;
     a value becomes a card (title = item description or key; phone/url clickable; image shown); a
     table becomes a compact list of rows. Only public items are read, through the public endpoint.
     To make phone and link values clickable, **`url` and `phone` become value types too** (today they
     exist for columns only): domain validation, admin value editor, MCP `data_set_value` description
     and the guide follow *(my call)*.
2. **Multi-page sites.** Every `*.html` file in the root of a version is a page:
   `index.html` → `/`, `contacts.html` → `/contacts`. Visiting `/contacts` shows our header with
   `contacts.html` in the sandboxed frame. Links between pages are `<a href="/contacts"
   target="_top">` so the browser address and Back button work. The whole site is published at once.
3. **One admin section "Site"** with two tabs:
   - **Pages**: a green "Live now" block (which version, note, who/when, "via MCP" badge, its pages as
     links, "Open site ↗", "Return to the built-in page"); a yellow "Drafts" block (each draft with
     note, author, MCP badge, page count, "Preview and publish"); a collapsed "Archive" with "Restore"
     (= publish again); "Upload zip" in the header.
   - **Built-in page**: community name, welcome text, default language, the "show under the quarter"
     picker (public data items, ordered), and a live mini preview.
   The old "Site" and "Site page" menu items become one "Site" item; old URLs redirect.
4. **Preview with page switching**: the draft preview shows our header layout with the frame and a
   page switcher (tabs or select) over the draft's pages; links inside the draft navigate within the
   preview.
5. **Agent support**: MCP tool results list a version's pages with their site paths; the page guide
   documents multi-page sites, reserved names and `target="_top"` links; `page_get_version` and
   `page_update_draft` already let the agent read every page and change only some files (no new
   tools needed).

### Non-goals

- Publishing single pages separately (approach 2 below), page-level permissions, nested paths
  (`/news/2026`), per-page titles in our header beyond the community name.
- Editing pages in the admin panel (pages come from the agent or a zip).
- An uploaded site picture: it is **removed** from the settings and the built-in page (owner agreed
  with the proposal; custom looks are done with published pages now). Removed: the
  `PUT/DELETE /api/v1/admin/site/picture` routes, `SetPicture`/`RemovePicture`, `pictureUrl` in the
  site API and DTOs, the picture clause in `AssetIsPublic`, and the picture UI. A stored
  `pictureAssetId` in the settings JSON is ignored (no migration); old audit entries keep their
  readable names. The builder greps for any other use before removing.

### Anti-goals

- Any page name taking over an app route (`/admin`, `/login`, …).
- A published link pointing to a page that is not published (cannot happen: whole-site publishing).
- The built-in page breaking or overflowing with many houses, a long community name, or on a phone.
- Private data showing on the built-in page.

## 3. Approaches considered

Multi-page structure — chosen by the owner: **pages are the HTML files of one site version**.

| Approach | Trade-off | Decision |
|---|---|---|
| **1. Pages = root `*.html` files of one version; whole site published together** | Links never break; same version/preview/publish flow; agent edits stay one draft. Changing one page means a new version (cheap with `page_copy_to_draft` + `page_update_draft`). | **Chosen** |
| 2. Each page its own object with own versions and publishing | Publish one page alone, but links break between published/unpublished pages and the UX becomes "which version of which page is live". | Rejected |
| 3. One page with `#sections` | No server changes, but no real addresses. | Rejected |

Admin layout — chosen by the owner from mockups: **tabs (A)** over a single dashboard (B).
Home page look — chosen: **C1** (yard + front street) over A (evening skyline), B (light house cards)
and C2 (blocks between streets).

## 4. Pages and routing

- **Page name** = a root-level `*.html` file name without extension; `index` is the home page `/`.
  Allowed names: `^[a-z0-9][a-z0-9-]{0,62}$`. Upper-case or other characters in a root `.html` name
  → validation error `files: invalid_page_name` (sub-folder `.html` files are allowed as assets, not
  pages).
- **Reserved names** (rejected with `files: reserved_page_name`): `admin`, `login`, `register`,
  `oauth`, `site`, `api`, `mcp`, `healthz`, `assets`, `telegram`, `favicon`, `robots`, `sitemap`,
  plus anything starting with `.`. The builder keeps this list next to the SPA route table and adds a
  test that every top-level SPA/server route is in it.
- **Server**: page files keep being served under `/site/...` as today. `GET /api/v1/site` gains
  `pages: ["/", "/contacts", ...]` for the published version (empty when none) in addition to
  `pageUrl`.
- **SPA**: a catch-all public route `/:page` (after all named routes) shows the shell + frame for
  `/site/<page>.html` when `<page>` is in `pages`; otherwise the existing not-found page. `/` behaves
  as today (published `index.html`, else the built-in page).
- The frame keeps the sandbox and CSP of part 2 unchanged; `?lang=`, `homelander:height` and
  `homelander:lang` are unchanged.
- **Links inside a draft preview** *(my call)*: a page's `<a href="/contacts" target="_top">` would
  leave the preview for the live site. So, **only for preview responses**, the server injects one small
  inline script into every served `.html` (the page CSP already allows inline scripts). It catches
  clicks on same-site links whose path is a page of that draft and, instead of navigating, posts
  `{ type: "homelander:navigate", path: "/contacts" }` to the parent. The preview shell accepts it
  only from its own frame and switches the page. Published pages get no injected script; links there
  navigate the top window as written. Agents write links one way for both.
- **Language across pages**: the chosen language is already kept in `localStorage` by the web app, so
  a `target="_top"` navigation keeps it; page paths do not carry `?lang=`.
- **Old versions**: page-name rules apply to new files (`CreateDraft`, `PutFiles`, zip upload). In any
  version, a root `.html` whose name breaks the rules or is reserved is simply **not a page** (still
  served as a file, no route). Restore and copy-to-draft are not blocked *(my call)*.

## 5. Built-in page details

- Header component used by the built-in page **and** by published pages (shell), so both get the
  fixed account control.
- Community name in the header: shown up to one line with ellipsis; the hero shows it in full.
- Quarter: SVG drawn in React from `GET /api/v1/register/options` houses (non-archived, sorted) — the
  builder may add `floors` to that response. House block height from floors (clamped), colours from a
  fixed palette, labels from house names (truncated with a tooltip on hover/focus). Layout: columns =
  `ceil(sqrt(n × 1.8))`, rows = `ceil(n / columns)`, empty cells get trees. The mockup at
  `.superpowers/brainstorm/.../home-iso-v3.html` is a sketch, not a spec — the builder polishes it.
- Data cards: site settings gain `homeItems: string[]` (ordered keys). The page loads
  `GET /api/v1/public/data` once and renders the listed keys present in it; missing or private keys
  are skipped silently.
- Accessibility: the quarter has an `aria-label` listing the house names; cards are a list.

## 6. Admin section "Site"

- Route `/admin/site` with tabs `?tab=pages` (default) and `?tab=home`; `/admin/pages` and
  `/admin/pages/:id` keep working (`/admin/pages` redirects to `/admin/site?tab=pages`; the preview
  page stays at `/admin/pages/:id` because MCP returns that link).
- Pages tab as in goal 3. Version cards show their pages as chips (`/`, `/contacts`); the live one's
  chips link to the real site. Empty states: "Nothing published — residents see the built-in page" and
  "No drafts. Ask your AI agent or upload a zip".
- Built-in page tab as in goal 3; the data picker lists **public** items only (with a hint when the
  data space has none) and allows reordering; saving writes `homeItems`.
- Preview page (`/admin/pages/:id`): status, note, author, actions (Publish with confirm, Copy to
  draft, Delete), and the preview in the real shell with a page switcher.

## 7. Agent / MCP

- `page_list_versions` and `page_get_version` results gain `pages: [{name, path}]`.
- `page_create_draft` / `page_update_draft` validation errors for page names come back as readable
  tool errors (`files: reserved_page_name (admin.html)`).
- `data_set_value` accepts `url` and `phone` value types.
- `guide.md` gets a "Several pages" section: file naming, reserved names, `target="_top"` links,
  shared CSS/JS between pages, how to edit an existing site (list → get → copy to draft → update →
  give the admin the link).

## 8. Constraints

- Every schema change is a new migration (site settings are JSON in `settings`, so `homeItems` likely
  needs none).
- i18n for all new UI (uk default, en). Repository is public: no real addresses or phones in fixtures.
- Existing tests for pages, site settings and MCP stay green; the removed picture upload takes its
  tests and route with it (`PUT/DELETE /api/v1/admin/site/picture`), and the audit action names stay
  readable for old entries.

## 9. Left to the builder

Visual polish of the quarter and cards within direction C1; the exact header/menu components; where
the mini preview sits in the Built-in page tab; the page switcher control; zip upload UI details;
whether `floors` comes from `register/options` or a new public endpoint.
