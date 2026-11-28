# Resident cabinet, data visibility, base modules and the information feed — design

- **Date:** 2026-11-28
- **Status:** Direction agreed with the owner in a brainstorm (2026-11-28); the owner delegated the technical
  calls ("decide on best practice"), which are listed in §8
- **Owner:** Igor Artasevych
- **Builds on:** [M0 + M1 design](2026-11-27-m0-m1-foundation-and-identity-design.md),
  [M2 design](2026-11-27-m2-chats-labels-and-bot-registration-design.md),
  [site data, pages and MCP](2026-11-27-site-data-pages-and-mcp-design.md),
  [site redesign](2026-11-27-site-redesign-and-multipage-design.md)

## 1. Intent

Residents can register and log in, but on the web they have nothing to do: after login they land on
the public site with an account menu that lists their apartments. All interaction is in Telegram,
and for some things (open the gate, look at the inverter, vote, see how a money collection is going)
a chat is awkward. The owner wants a **resident cabinet**: our own app for residents, installable as a
**PWA**, where base modules show their part.

The bigger picture, agreed in the brainstorm (only part of it is built now, see §3):

- **Base modules** are the most popular use cases of a resident community, built by us: an
  information feed, projects with votes, money collections, later requests/complaints with house
  dispatchers. An admin **turns each module on or off**. Later WASM modules (M3) follow the same
  shape.
- A module has **its mechanics**, **its own data** (published into the data space under its own
  keys), and **our standard UI** in three places: admin settings, a cabinet section with actions, and
  the Telegram bot. Telegram is an equal channel to the cabinet, not a lesser one.
- The **public site stays AI-built** (pages through MCP, published by an admin). It is for residents
  too, not only guests: it shows module data any way the community likes (a map of a garden
  society, project progress). AI pages only **read**; actions link into the cabinet. Residents never
  write through an AI page.
- The **cabinet and the admin panel are not AI-built.** The system must be configurable without an AI
  agent.
- **Data visibility** has three levels: **everyone / residents / admins**. The same site page shows
  more detail when a resident is logged in ("60% collected" for everyone; which apartments paid for
  residents). Visibility is **community-wide**, not per house (the owner sees no case for per-house
  visibility; houses seeing each other's progress is a plus).

## 2. Goals, non-goals, anti-goals

### Goals (this design)

1. **Three-level data visibility** replacing the public flag, with resident-level data delivered to
   site pages when a resident (or admin) is logged in.
2. **Resident cabinet** at `/my`: our React app, mobile-first, with "My home" and a place for module
   sections. Installable as a PWA named after the community.
3. **One-tap login from the bot**: the bot hands a verified user a short-lived link that logs them in
   and opens a cabinet page. Bot messages can deep-link into the cabinet.
4. **Base module framework (minimal)**: a registry of built-in modules, an admin "Modules" section to
   turn them on and off, module-owned data items.
5. **Information feed** — the first base module: admins write posts; posts show in the cabinet feed;
   on publish they are also posted to Telegram through chosen **labels**; the feed's latest posts are
   published as a module-owned data item so the AI site can show them.

### Non-goals (now)

- Projects, votes, money collections, requests/complaints: each needs its own brainstorm with the
  owner. Nothing here should pre-build them beyond the module framework.
- **Important announcements** (personal delivery to every resident, read receipts): a separate later
  module. The feed is the everyday stream, not that.
- Residents adding or arranging widgets themselves; admin-defined dashboards.
- Web Push notifications. Telegram stays the push channel. (The service worker added here must not
  block adding push later.)
- Editing or deleting already-sent Telegram messages when a post is edited or deleted.
- Posting to the feed through MCP. Scheduled posts. Pinned posts. Comments and reactions.
- Per-house visibility. Roles beyond the Resident/Admin flags.
- Custom non-apartment objects in houses (commercial premises) — later.

### Anti-goals

- A generic UI/widget constructor. Flexibility comes from base modules and the AI site, not from a
  builder.
- Anything that makes the system impossible to set up without an AI agent.
- Resident-only or admin-only data reaching a guest, a page draft opened by a non-admin, or anything
  outside our origin through our own code.

## 3. Shape of the work: three parts, three PRs

| Part | What | Depends on | Migration |
|---|---|---|---|
| 1 | Data visibility (§4) | — | `0007_data_visibility.sql` |
| 2 | Cabinet shell, PWA, login links from the bot (§5) | — | `0008_login_links.sql` |
| 3 | Base modules + information feed (§6) | 1 and 2 | `0009_modules_feed.sql` |

Parts 1 and 2 are independent and may be built in parallel. Part 3 is built on top of both. Migration
numbers are fixed here so parallel branches do not collide.

## 4. Part 1 — Data visibility

### Model

- `data_items.is_public` becomes `visibility TEXT NOT NULL CHECK (visibility IN ('public', 'residents',
  'admins'))`. Migration: `is_public = 1` → `public`, `0` → `admins` (so nothing becomes more visible
  than today). SQLite: add the column, copy, drop the old column (or rebuild the table); keep indexes
  and foreign keys intact.
- Domain: `DataItem.Public bool` → `DataItem.Visibility` (`VisPublic`, `VisResidents`, `VisAdmins`).
  `DataInput.Public *bool` → `Visibility *Visibility`. Audit action `data.public_set` is replaced by
  `data.visibility_set` with `{"from": ..., "to": ...}`; old audit rows keep their action name and must
  still render in the audit UI.
- **Who is a "resident" for reading:** an `active` user with `is_resident = 1`. Admins (`is_admin = 1`,
  active) see `residents` items too. A disabled/pending user sees only `public`.

### Endpoints

- `GET /api/v1/public/data` and `/public/data/{key}` — unchanged: `public` items only, CORS, 30 s
  cache, rate limit.
- **New** `GET /api/v1/me/data` — session required; returns `public` + `residents` items for a
  resident or admin (same shape as the public endpoint); for any other signed-in user only `public`.
  `Cache-Control: private, no-store`. Not CORS-enabled (only our shell calls it).
- `admins` items are never returned by either endpoint (admins read them in the panel and via MCP).

### Delivering resident data to a site page

The page is in a sandboxed frame with an opaque origin and no cookies, so it cannot call
`/api/v1/me/data` itself. Our shell does:

1. The page posts `{ type: 'homelander:ready' }` to its parent when its script starts listening.
2. When the viewer is signed in, `PageFrame` fetches `/api/v1/me/data` and posts
   `{ type: 'homelander:data', viewer: { signedIn: true, resident: bool, admin: bool }, items }` to the
   frame (after `ready`, and again after a language change or a re-fetch). For a guest it posts
   `{ type: 'homelander:data', viewer: { signedIn: false }, items: null }` so the page knows not to wait.
3. The page keeps reading `/api/v1/public/data` itself for public items (works for guests and is the
   fallback), and merges `items` from the message when they arrive.

- **Previews:** a draft preview is opened by admins only, so the shell may post admin-visible
  `residents` data there too. Never post data into a frame whose `src` is not one of our page URLs.
- **Images in `residents` items:** the frame's `<img>` requests carry no cookies (opaque origin;
  `SameSite=Strict`). `/api/v1/me/data` therefore returns image values as **signed, expiring URLs**
  (`/api/v1/assets/{id}?exp=<unix>&sig=<hmac>`, HMAC with the app secret over id+exp, ~1 hour). The
  asset handler serves an asset when it is referenced by a public item, **or** the signature is
  valid. Public items keep plain URLs.
- Exfiltration note (accepted): a published page's JS sees resident data. Its CSP blocks `fetch`
  to other hosts, but it could still open a popup with data in the URL. Pages are written through
  MCP by an admin's agent and published by an admin who can already read all data; we accept this
  and say it in the admin UI help text next to the visibility control ("pages published on the site
  can show residents-level data to logged-in residents").

### Admin panel and MCP

- Data section: the public switch becomes a three-option control (segmented: Everyone / Residents /
  Admins) in the list, the item page and the create modal; the list can filter by visibility.
- MCP: `data_set_public` is replaced by `data_set_visibility(key, visibility)`; `data_catalog` and
  `data_get` report `visibility`; create tools accept `visibility` (default `admins`). Update
  `internal/mcpserver/guide.md` §3 with the `ready`/`data` messages, the viewer object, an example
  that renders a public summary and a resident detail, and the rule "design for both states".
- `docs/mcp.md` updated accordingly.

## 5. Part 2 — Cabinet shell, PWA, login links

### Who gets the cabinet

- `/my` requires a session. Guest → `/login?next=/my...`.
- Any active user may open it. A user without residencies (e.g. an admin who is not a resident) sees
  the cabinet with an empty "My home" explaining that no apartment is linked.
- After login, a non-admin goes to `next` or `/my` (today: `/`). Admins still go to `/admin`.
- The site header's account menu gets "My cabinet" (`/my`) above "Admin panel".
- `my` is added to reserved page names (`web/src/lib/reservedPages.ts` and `ReservedPageNames` in
  `internal/domain/page.go`). If a published site already has a page named `my`, the app route wins;
  note it in the PR.

### Layout

- Mantine `AppShell`, mobile-first: **bottom navigation on phones**, a sidebar on wide screens.
  Header: community name, account menu, language switch (reuse `SiteHeader` parts).
- Navigation entries come from a small front-end registry: `My home` always; module sections (Part 3)
  appear only when their module is enabled (from an API list, see §6). A link "Site" back to `/`.
- `/my` (index) is the **home of the cabinet**: in Part 2 it shows "My home"; in Part 3 the feed's
  latest posts appear on top. Keep the index a composition of section cards so later modules add a
  card without reworking it.

### My home

- For each residency: house name, address, floors, apartment number, residency status (localized).
- Profile block: name, phone (masked except the last 4 digits), Telegram linked or not, language.
  Read-only in this part; "ask an admin to change" hint.
- No list of other residents (privacy; decide later with the owner).

### PWA

- **Manifest served by the backend** at `/site.webmanifest` (replacing the static file): `name` =
  community name (localized to the site's default language), `short_name` ≤ 12 chars (cut with an
  ellipsis-free rule: first word or first 12 chars), `start_url: "/my"`, `scope: "/"`,
  `display: "standalone"`, `theme_color`/`background_color` matching the Mantine theme, existing
  icons (custom community icon is a later item). Short cache (5 min).
- A **hand-written minimal service worker** (`/sw.js`, no extra dependency): installs, serves an
  offline fallback page for navigations when the network fails, and never caches `/api/`, `/site/`,
  `/mcp`, `/oauth`. Registered only in production builds. It must leave room for Web Push later.
- iOS: `apple-mobile-web-app-capable`, title, status-bar style; `apple-touch-icon` exists.
- An "Install the app" card in the cabinet: on Chromium uses `beforeinstallprompt`; on iOS Safari
  shows the "Share → Add to Home Screen" instruction; hidden when already running standalone
  (`display-mode: standalone`) or dismissed (remember in localStorage).

### Login links from the bot

The PWA keeps the session for `SESSION_IDLE_TTL`/`SESSION_MAX_TTL` (defaults 12 h / 7 d); typing a
phone and a code every week is friction. The bot already knows the verified user, so:

- **Bot command `/cabinet`** (added to the private-chat command menu, localized) for an active, linked
  user: replies with an inline URL button "Open cabinet" → `/login/link?t=<token>`. Unknown or
  not-active users get the usual help/registration answer.
- `login_links` table (migration 0008): `token_hash` (HMAC like other tokens), `user_id`, `next`
  (validated relative path starting with `/my`), `expires_at` (**5 minutes**), `used_at`,
  `created_at`. Single use. Rate-limited per user.
- `GET /login/link?t=…` is a SPA route; it calls `POST /api/v1/auth/link` with the token, which opens
  a normal session (same cookie, same audit `auth.login` with `{"via": "bot_link"}`) and redirects
  to `next`. Expired/used → the login page with a message and a "Send a new link" hint.
- Telegram may pre-fetch URLs for previews: the GET page must not consume the token; only the POST
  from our page does.
- Domain service gets `CreateLoginLink(ctx, userID, next)` so other code (Part 3 feed posts, later
  modules) can deep-link a **specific user** into the cabinet. Messages to **group chats** never carry
  login tokens: they link to plain `/my/...` URLs (a guest is sent through `/login?next=...`).

## 6. Part 3 — Base modules and the information feed

### Module framework (minimal, YAGNI)

- Built-in modules are Go code registered in a registry: `code` (`feed`), localized name and
  description keys, default enabled state. No dynamic loading.
- `modules` table (migration 0009): `code TEXT PRIMARY KEY`, `enabled`, `settings TEXT (json)`,
  `updated_by`, `updated_at`. A row is created lazily; missing row = the module's default.
- Admin **Modules** section (`/admin/modules`): one card per module with name, description, on/off
  switch, link to its settings. Audit `module.enabled` / `module.disabled` / `module.settings_saved`.
- `GET /api/v1/modules` (session): enabled modules with their cabinet nav entries for the current
  user; the cabinet builds its navigation from it.
- Default state per module in the registry; the **feed defaults to on** (it is empty and invisible
  until an admin publishes a post, so turning it on for existing deployments has no side effect).
- A disabled module: its cabinet section and API return 404, its admin page shows only "turned off"
  with the switch. Its data stays **in the database but is not served**: module-owned items of a
  disabled module are left out of `/public/data`, `/public/data/{key}`, `/me/data`, and their assets
  are not served through the public/signed asset paths. Admins still see them in the Data section
  (marked "module off"). Turning the module back on serves them again.
- The registry entry has room for bot commands, but **no module has bot commands in this design**
  (the feed reaches Telegram through labels only). Don't build bot-command routing for modules yet.
- **Module-owned data items:** `data_items.owner TEXT` (nullable; `module:feed`). Owned items are
  written only by their module. In the Data section and in MCP they are read-only except
  `visibility`; they show "managed by module <name>". Deleting an owned item is refused.
- **Module key prefixes are reserved:** a module with code `feed` owns every key starting with
  `feed.`. Admins and MCP may not create keys under a registered module's prefix (validation error
  `key_reserved`). Migration 0009 renames any existing item whose key starts with `feed.` to
  `custom.feed.…` (so nothing is lost or taken over silently; the PR notes it).
- A module creates its owned items when it is **enabled** (and at startup for enabled modules whose
  items are missing), so `feed.posts` exists as an empty table from the start and agents can build
  pages against it. This is the
  shape WASM modules (M3) will reuse ("modules write only their own keys"; see the module service
  accounts decision — audit actor for module writes can stay the system actor for now).

### Feed: what a post is

- `feed_posts`: `id`, `body` (plain text, ≤ 4000 chars; with an image ≤ 1000 chars because of
  Telegram's caption limit), optional `image_asset_id`, `state` (`draft` / `published`),
  `published_at`, `labels` (json array of label ids chosen at publish), `created_by`, `updated_by`,
  `created_at`, `updated_at`, `deleted_at` (soft delete).
- Text formatting: plain text with line breaks; URLs auto-linked in the cabinet; sent to Telegram as
  plain text (no parse mode), so no escaping issues. Rich formatting is a later item.
- One language per post (the author's); no per-locale variants.

### Admin side

- Feed settings page (`/admin/modules/feed`): **default labels** for new posts.
- Posts list (newest first): body excerpt, state, published date, per-label delivery status.
- Post editor: body, optional image (reuse the data image upload/asset storage), "Also post to"
  multi-select of labels (prefilled with the defaults; only labels that reach an accepted chat are
  offered; empty = cabinet only), preview of how it reads. Actions: Save draft, Publish.
- **Publish** is one-time: it sets `published` and fixes the chosen labels (a published post cannot
  go back to draft or be sent to more labels). It resolves the destinations of **all chosen labels
  together, removing duplicates** (a chat or topic that carries two chosen labels gets the post
  once), and enqueues one outbox message per destination (origin `feed:<id>`). The Telegram message
  has the text (or photo with caption) and a URL button "Open in cabinet" →
  `<PUBLIC_URL>/my/feed/<id>` (only when `PUBLIC_URL` is https, like the registration button).
- Delivery status is shown **per destination** (chat / topic title, pending / sent / failed with the
  error), from the outbox rows of that origin. The outbox drops finished rows after its retention
  (30 days); for older posts the status block says delivery details are no longer kept.
- **Edit after publish** changes the cabinet and site only; the UI says Telegram copies are not
  changed. **Delete** soft-deletes (gone from cabinet and site), same note.
- Audit: `feed.post_created`, `feed.post_published`, `feed.post_updated`, `feed.post_deleted`.

### Telegram: photos

`OutMessage` gains an optional image (asset id). The outbox payload stores it; the Telegram adapter
sends `sendPhoto` with the uploaded bytes and the text as caption (plain), buttons as now. The
outbox worker reads the asset bytes from storage at send time. A missing asset → permanent failure
with a clear error.

### Cabinet side

- `/my/feed`: published posts newest first, paginated (cursor, 20 per page), with date, author's
  first name, text, image. `/my/feed/:id`: one post (deep-link target).
- The cabinet home shows the latest 3 posts on top with "All posts".
- Only active users see the feed (it is the cabinet). No read/unread state in this part.

### The feed on the site

- The module keeps a module-owned table item **`feed.posts`**: columns `id` (number), `date`
  (text, ISO), `text` (text), `image` (image), `url` (url → `<PUBLIC_URL>/my/feed/<id>`, absolute
  because url values must be absolute http(s)); rows = the latest
  20 published, not deleted posts, rewritten on every publish/edit/delete.
- Default visibility **`residents`** (an admin may make it `public` or `admins`).
- Mention `feed.posts` in the MCP guide as an example of module data.

## 7. Testing and verification

- Follow the project's existing patterns: domain service tests with the test env, httpapi tests,
  web tests with `mockApi`/`renderApp`, en/uk locale parity checks.
- Part 1: migration maps old flags; endpoint tests for guest/resident/admin/disabled; signed asset URL
  (valid, expired, tampered); PageFrame posts data only after `ready` and only to our frame.
- Part 2: route guard, login redirect rules, login link (single use, expiry, no consumption on GET,
  `next` validation, rate limit), manifest content, reserved name.
- Part 3: module on/off effects (API 404, nav), owned data item read-only rules, publish → outbox
  rows per label destination, photo send path in the Telegram adapter (fake API), `feed.posts`
  regeneration, cabinet feed pagination.
- Each part is checked in the running app (desktop and a mobile viewport) before its PR is ready.

## 8. Decisions the builder made (owner may revisit)

- Cabinet path `/my`; bot command `/cabinet`; login links live 5 minutes.
- `is_public = 0` migrates to `admins`, not `residents`.
- Resident data reaches pages via `postMessage` from our shell; resident images via signed URLs.
- Feed posts are plain text with an optional image; Telegram copies are not edited or deleted.
- `feed.posts` defaults to `residents` visibility.
- Modules are a code registry plus an on/off table; no dynamic loading yet.
- The feed is on by default; a disabled module's data is kept but not served.
- Module key prefixes (`feed.`) are reserved; clashing existing keys move to `custom.feed.…`.
- A post goes once to each chat/topic even when several chosen labels reach it; status is per
  destination; publishing is one-time.
