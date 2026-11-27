# Site data, agent-built home page, and the MCP server — design

- **Date:** 2026-11-27
- **Status:** Approved by the owner (2026-11-27)
- **Owner:** Igor Artasevych
- **Tracks:** issue #36 (to be rewritten to this purpose)
- **Builds on:** [M0 + M1 design](2026-11-27-m0-m1-foundation-and-identity-design.md),
  [M2 design](2026-11-27-m2-chats-labels-and-bot-registration-design.md), [ADR-0001](../../adr/0001-stack-and-process-shape.md)

## 1. Intent

The home page (`/`) today is fixed: community name, a picture or the built-in houses illustration,
and the welcome text, all from site settings. The owner wants the page to be **designed by an AI
agent** (Claude or similar) — any layout, images, PNG/APNG animations — and to **show live community
data**: important phone numbers, the address, later the outage schedule and other module output.

Two things are needed for that, and a third connects them:

1. A **data space** inside the system: named values and tables (in the spirit of n8n Data Tables)
   that admins, the agent and, later, modules write to, and that the page and modules read. Each
   item is either private or **public**.
2. **Agent-built pages**: the agent writes a page (HTML, CSS, JS and its own assets); an admin
   previews and publishes it. The published page **reads public data at view time**, so when an
   admin changes a phone number, the page shows the new number without being rebuilt.
3. An **MCP server** through which the agent does both: discovers which data exists and how to load
   it, creates and fills data, and writes page drafts.

Issue #36 framed MCP as "manage Homelander through an AI assistant" in general. This design narrows
it to the purpose above; general admin work through MCP can grow from the same server later.

## 2. Goals, non-goals, anti-goals

### Goals

1. **Data space.** Items addressed by an English path key (`house.address`, `contacts`), of two
   kinds: a **value** and a **table** (columns + rows). Each item has a description (for people and
   for the agent) and a **public** flag.
2. **Admin panel: Data section** — list, create, edit, delete values and tables; edit table rows;
   toggle public.
3. **Public data endpoint** — returns public items only; rate-limited per IP; short HTTP caching;
   readable from the sandboxed page (CORS).
4. **Pages.** Page **versions** made of files (an `index.html` plus CSS/JS/images). A version is a
   **draft** until an admin publishes it. Admins preview drafts, publish, roll back to an earlier
   version, or go back to the built-in page.
5. **Home page = our shell + sandboxed iframe.** Our React header (community name, login / account,
   language) stays; the published page renders below it in an `<iframe sandbox>` that has no access
   to sessions or the admin API. With nothing published, the current built-in page is shown.
6. **MCP server** inside the Go binary at `/mcp` (Streamable HTTP), tools for the data space and for
   page drafts, plus a guide that tells the agent how a page loads public data.
7. **MCP access control.** A new user flag **MCP**, effective only together with **Admin**. Agents
   connect through **OAuth** using our normal login (phone + Telegram code) and a consent screen; the
   token acts as that admin. Connected agents are listed on the user card and can be revoked.
8. **Audit.** Every change to data and pages is audited. Actions done through MCP are recorded as
   the admin, marked "via MCP" with the agent's client name.

### Non-goals (now)

- **Agent publishes the page.** Never: publishing is a human action in the panel.
- **RBAC / ACL / per-item permissions.** Writers are admins (panel) and admin-backed agents (MCP).
  Residents and guests only read public data through the page.
- **Module writers and module service accounts.** When modules arrive (M3, epic #4) each connected
  module gets its own auto-created **service account** and writes only to its own keys
  (e.g. `power.schedule`); the audit log then shows "module X". This design keeps the actor model
  open for that (section 7) but builds nothing for modules.
- Other MCP tools (users, applications, chats, labels). Same server, later.
- Pages other than the home page, per-house pages, private (logged-in-only) data on the page.
- External resources on the page (CDNs, web fonts, analytics). See section 5.4.

### Anti-goals (a failure even if it works)

- Code written by the agent can read a session cookie, call the admin API, or run on the same
  origin as `/admin` with access to it.
- A private item, or an asset only a private item refers to, is reachable without logging in.
- A bad agent draft reaches visitors without an admin pressing Publish.
- Something changed through MCP shows in the audit log as if the admin clicked it by hand.

## 3. Approaches considered

How to isolate the agent's free page — chosen by the owner: **shell + sandboxed iframe**.

| Approach | Trade-off | Decision |
|---|---|---|
| **Our React shell + `<iframe sandbox="allow-scripts …">` without `allow-same-origin`** | Same domain, no DNS work; the frame gets an opaque origin, so it cannot touch cookies or the API with the user's session; our login/account header stays. Height must be synced via `postMessage`; fully "edge-to-edge" designs are slightly constrained. | **Chosen** |
| Separate subdomain for the page (`www.` vs `app.`) | Strongest isolation, a normal top-level page. Needs a second domain on Railway and DNS, CORS for data, and the agent must draw the login button itself. | Rejected: setup cost, loses our header |
| Page served on `/` with a `CSP: sandbox` header, no iframe | No iframe, no second domain, but our login/account header disappears. | Rejected |

Earlier decisions in the same conversation (owner): a **free page** rather than a fixed set of
blocks; **both values and tables** in the data space; **a separate MCP flag** for admins rather than
all admins; **OAuth** through our own login.

Where the MCP server lives — builder's choice made here: **inside the one Go binary** at `/mcp`
using the official Go SDK (`github.com/modelcontextprotocol/go-sdk`), consistent with ADR-0001's
one-binary shape. A separate service would add nothing on one replica.

## 4. Data space

### 4.1 Items

- **Key**: lower-case path, `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`, max 100 chars, unique across
  values and tables. A key cannot be renamed (pages and modules refer to it); delete and recreate.
- **Kind**: `value` or `table`.
- **Description**: plain text, shown in the panel and in the agent's catalog.
- **Public**: bool, default **false**.
- Created/updated by and at (user id; later also module).

### 4.2 Value types

`text`, `localized_text` (uk/en, same shape as `LocalizedText` in site settings), `number`, `bool`,
`image` (an asset id; public output is its URL), `json` (any JSON, max 64 KiB).

### 4.3 Tables

- Ordered **columns**: name (same rules as a key segment), label, type (`text`, `localized_text`,
  `number`, `bool`, `image`, `url`, `phone`). Adding a column is allowed with data present; removing
  a column drops its values; changing a column type is allowed only on an empty table *(my call:
  keeps validation simple)*.
- **Rows**: an id, a position (manual order), and a value per column. Up to **1000 rows** per
  table *(my call)*.
- Storage: rows as one JSON object per row in SQLite, validated against the columns in the domain.

### 4.4 Public output

- `GET /api/v1/public/data` — all public items in one JSON object keyed by item key. One request
  is what a page needs in practice.
- `GET /api/v1/public/data/{key}` — one public item. Private or missing → `404` (same response for
  both).
- Shape: value → `{ "kind": "value", "type": "...", "value": ... }`; table →
  `{ "kind": "table", "columns": [...], "rows": [ { "id": ..., "<col>": ... } ] }`. Images become
  `"/api/v1/public/assets/<id>"` URLs.
- Headers: `Access-Control-Allow-Origin: *` (the sandboxed frame has origin `null`; the data is
  public anyway); `Cache-Control: public, max-age=30`; `ETag`.
- **Rate limit** per client IP with the existing `internal/ratelimit` (e.g. 120 requests/minute;
  exact numbers are the builder's), `429` when exceeded. "Only from our site" cannot be enforced —
  any request can be forged — so the protection is: only public items leave, plus the limit and
  caching. Cloudflare only helps if the domain is proxied through it; not required.

### 4.5 Assets must not leak

Today `GET /api/v1/assets/{id}` serves **any** asset by id to anyone. With private data that can
hold images, that is no longer acceptable. A new public asset route serves only assets referenced
by: site settings (picture), a public item, or a page version's files (published, or a draft via a
preview token). Admin-only assets are served through an admin route. The old route is migrated to
these rules *(builder decides whether to keep the path)*.

## 5. Pages

### 5.1 Versions

- A **page version**: id, a set of **files** (path → content type + bytes), created by (user, and
  "via MCP" if so), created at, a short note, status `draft` / `published` / `archived`.
- `index.html` is required. Allowed file types: html, css, js, json, svg, png (incl. APNG), jpeg,
  webp, gif, woff2. Size limits per file and per version (e.g. 5 MiB / 20 MiB) — builder's numbers.
- Exactly one version is published, or none (→ built-in page). Publishing archives the previous
  one; rollback = publish an archived version again. Drafts are editable; published/archived
  versions are not (copy to a new draft instead).
- Versions are kept; deleting drafts and archived versions is allowed in the panel.

### 5.2 Serving

- Published files at `/site/{path}` *(path prefix is the builder's call)*; a draft at
  `/site/preview/{token}/{path}` where the token is a short-lived signed preview token issued to an
  admin. Drafts are never reachable without a token.
- Response headers for page files: a page-specific CSP replacing the global one —
  `sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation;
  default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';
  img-src 'self' data: blob:; connect-src 'self'; frame-ancestors 'self'` — so the page works even
  if opened directly, and can only be framed by our own site. Builder may adjust the exact list but
  not loosen the isolation or allow other origins.

### 5.3 Home page shell

- `HomePage` asks the public site endpoint whether a page is published. If yes: header as today +
  `<iframe src="/site/index.html?lang=uk" sandbox="allow-scripts allow-popups
  allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation">` full width. If no:
  the current built-in page.
- **Height**: the page posts `{ type: "homelander:height", height }`; the shell accepts it only
  from its own iframe's `contentWindow` and sets the height. The agent guide includes the snippet.
- **Language**: the shell passes the current language in `?lang=` and, on switch, posts
  `{ type: "homelander:lang", lang }` into the frame.
- Links inside the page to `/login`, `/register` work as top navigation on click.
- The global `frame-ancestors 'none'` stays for everything except page files.

### 5.4 No external resources

The page loads only its own files and our public data (CSP above). No CDNs, web fonts or trackers:
residents' visits are not reported to third parties, and the page cannot break when a CDN changes.
The agent uploads fonts/libraries as files if it needs them. *(My call; easy to relax later for a
listed set of hosts.)*

### 5.5 Admin panel: Pages section

List of versions with status, author ("via MCP" badge), date, note; **Preview** (opens the draft in
the same shell + iframe layout), **Publish**, **Unpublish (back to built-in page)**, **Copy to
draft**, **Delete**. Publishing asks for confirmation.

## 6. MCP server

### 6.1 Transport and auth

- `/mcp`, Streamable HTTP, official Go SDK.
- **OAuth 2.1 with PKCE**, Homelander as its own authorization server, following the current MCP
  authorization spec: protected-resource metadata, authorization-server metadata, and client
  registration in the form Claude's custom connectors use (dynamic client registration). The builder
  verifies the exact requirements against the current spec and Claude's connector docs.
- **Redirect URIs are allow-listed** *(my call)*: registration accepts only redirect URIs on a
  list from configuration (env). The default list covers Claude (claude.ai / Claude Desktop
  connector callback) and loopback redirects (`http://127.0.0.1:*`, `http://localhost:*`) for
  local clients such as Claude Code. Other agents (Cursor, ChatGPT, …) are added by extending the
  env list. Open registration is rejected: an attacker could register a client named "Claude"
  with their own redirect and send an admin a consent link.
- The consent screen shows the client's self-reported name **and the redirect host**.
- Authorization page in our web app: the admin logs in as usual (phone + Telegram code), sees "Allow
  *<client name>* to act as you in Homelander", approves. Only users with **Admin and MCP** both on
  can approve; others see why not.
- Access tokens short-lived, refresh tokens stored hashed; every token belongs to one user and one
  client. Each request re-checks that the user is active, Admin and MCP — removing either flag cuts
  the agent off immediately.
- Rate limit per token.

### 6.2 User flag and card

- New column `is_mcp` on users. The panel shows the MCP switch only when Admin is on; revoking Admin
  leaves `is_mcp` stored but ineffective *(my call; it reads as "off" in the UI)*.
- Any admin may switch MCP on or off for any admin, including themselves — the same trust as the
  Admin flag today. It is an opt-in switch, not an extra security gate; a stricter rule would need
  an owner/super-admin role, which does not exist yet *(my call)*.
- User card, new block **Connected agents**: client name, connected at, last used; **Revoke**
  (deletes its tokens).
- Audit: `user.mcp_granted`, `user.mcp_revoked`, `mcp.client_authorized`, `mcp.client_revoked`.

### 6.3 Tools (first set)

Data:
- `data_catalog` — all items: key, kind, type/columns, public, description.
- `data_get` (value or table with rows, paged), `data_set_value`, `data_create_table`,
  `data_update_table_columns`, `data_rows_upsert`, `data_rows_delete`, `data_set_public`,
  `data_delete`.
- `asset_upload` — base64 file → asset id (for `image` values/columns).

Pages:
- `page_guide` — how a page is built here: the sandbox and CSP rules, the public data URLs and
  response shapes, the height and language messages with ready snippets, file limits. Also exposed
  as an MCP resource / server instructions so the agent reads it first.
- `page_list_versions`, `page_get_version` (file list + contents of text files).
- `page_create_draft` (files + note) → draft id and a **link into the admin panel**
  (`/admin/pages/{id}`). It works only for a logged-in admin; the panel mints the preview token
  for that admin's session. The agent never holds a working draft URL, so a draft cannot reach
  anyone else *(my call: keeps the "no draft reaches visitors" anti-goal; the cost is that the
  agent cannot render its own draft — it relies on the admin's feedback)*;
  `page_update_draft` (put/delete files); `page_copy_to_draft`.
- No publish tool.

Errors come back as readable tool errors (validation messages in English).

## 7. Audit and actors

- `AuditEntry` keeps `ActorUserID`; new details field `via` (`"mcp"`) and `client` (client name)
  for MCP actions. The panel's audit view shows "*Name* via MCP (*client*)".
- New actions: `data.created`, `data.updated`, `data.deleted`, `data.public_set`,
  `data.rows_changed` (one entry per tool call/save, with counts, not per row), `page.draft_created`,
  `page.draft_updated`, `page.published`, `page.unpublished`, `page.deleted`, plus the MCP ones above.
- Room for later: an actor can become a module service account (non-goal now); the builder should
  not hard-code "actor is always a user with a phone" in new code paths.

## 8. Constraints

- One Go binary, SQLite via repository interfaces, every schema change a **new migration file**
  (next number after `0003`).
- i18n for all new UI text (uk default, en). MCP tool descriptions and errors in English.
- The repository is public: no real addresses or phones in fixtures, tests or the agent guide
  examples.
- One replica: in-memory rate limits are fine.
- Admin API pattern as today (`RequireAdmin`), audit for every change.

## 9. Order of work

1. **Data space** — storage, domain, admin API, Data section in the panel, public data endpoint,
   asset access rules (4.5).
2. **Pages** — versions, serving with CSP, preview tokens, Pages section, home page shell.
3. **MCP** — flag, OAuth, connected agents, tools, guide.

Each step is usable on its own: after 1, data can be edited; after 2, an admin can upload a page by
hand (zip upload in the Pages section — *my call, also useful for testing*); after 3, the agent does
it.

## 10. Left to the builder

Exact rate-limit numbers and size limits; route prefixes for page files; table/column storage
details; MCP SDK wiring and token formats; UI layout of the Data and Pages sections within the
existing admin style; how the zip upload is parsed.
