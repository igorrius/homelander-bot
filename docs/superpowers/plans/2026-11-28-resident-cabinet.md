# Resident cabinet: shell, PWA and bot login links — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give residents their own app at `/my` (mobile-first, installable as a PWA named after the
community) and let the bot log a verified user in with one tap.

**Architecture:** Backend: a `login_links` table and two `AuthService` methods (`CreateLoginLink`,
`UseLoginLink`) that share the session-opening code with the phone/code login; a bot `/cabinet`
command; `POST /api/v1/auth/link`, `GET /api/v1/me/home` and a generated `/site.webmanifest` in a new
`httpapi/cabinet.go`. Front end: a `CabinetLayout` (guard + `AppShell`, bottom navigation on phones,
sidebar on wide screens) whose navigation and home page are compositions of small registries that
Part 3 extends; a hand-written `sw.js` with an offline fallback; an install card.

**Tech Stack:** Go 1.27 (net/http, SQLite, gotgbot), React 19, Mantine 9, TanStack Query, react-router 8,
vitest.

**Spec:** `docs/superpowers/specs/2026-11-28-resident-cabinet-and-feed-design.md` §5 (with §1–§3, §7, §8).

## Global Constraints

- Docs, comments, commit messages in English. UI texts through locale files, uk (default) and en in
  parity: `web/src/locales/*.json`, `locales/*.json`.
- Migration file is exactly `internal/storage/sqlite/migrations/0008_login_links.sql`.
- Public repo: synthetic fixtures only (`+3805000000NN`, "House A", example.com).
- Stay out of Part 1: no `data_items` changes, no `/api/v1/me/data`, no PageFrame data messages,
  no MCP data tools, no admin Data pages.
- Login links: 5 minutes, single use, `next` is a relative path starting with `/my`, rate-limited per
  user; GET never consumes, only `POST /api/v1/auth/link`; audit `auth.login` with `{"via":"bot_link"}`.
- Group chats never get login tokens (only `CreateLoginLink` for a specific user mints them).
- Manifest: `start_url "/my"`, `scope "/"`, `display "standalone"`, theme colour `#0ca678`
  (Mantine teal-7, the app's primary shade), background `#ffffff`, `Cache-Control: public, max-age=300`.
- Service worker never handles `/api/`, `/site/`, `/mcp`, `/oauth`, `/telegram/`, `/healthz`; registered
  only when `import.meta.env.PROD`.

## Review Focus

1. A link opened in React StrictMode / double render must POST once (a second POST would burn the
   token and show "expired") — LoginLinkPage test counts POSTs.
2. A link opened while signed in as someone else must switch to the link's user (cookie replaced) —
   httpapi test logs in as A, redeems B's link, `/me` is B.
3. `next` tricks: `/my//evil.example`, `/myfoo`, `/my\x`, `https://…`, `/admin` → rejected (validation),
   `/my`, `/my/feed/3`, `/my?x=1` accepted — domain table test.
4. A user disabled after the link was minted must not get a session — domain test.
5. Community name empty / long Cyrillic name → manifest falls back to "Homelander" and `short_name` is
   rune-safe (≤ 12 runes) — httpapi manifest test.

---

### Task 1: Login link storage and domain service

**Files:**
- Create: `internal/storage/sqlite/migrations/0008_login_links.sql`
- Create: `internal/domain/loginlink.go`
- Modify: `internal/domain/users_service.go` (add `LoginLinks LoginLinkRepo` to `Repos`)
- Modify: `internal/domain/auth_service.go` (extract `openSession`, add link methods)
- Modify: `internal/storage/sqlite/auth.go`, `internal/storage/sqlite/repos.go`
- Test: `internal/storage/sqlite/auth_test.go`, `internal/domain/loginlink_test.go`

**Interfaces:**
- Produces:
  - `type LoginLink struct { ID, UserID int64; TokenHash []byte; Next string; ExpiresAt time.Time; UsedAt *time.Time; CreatedAt time.Time }`
  - `type LoginLinkRepo interface { Create(ctx, *LoginLink) error; GetByHash(ctx, []byte) (LoginLink, error); MarkUsed(ctx, id int64, at time.Time) (bool, error); DeleteExpiredBefore(ctx, time.Time) (int, error) }`
  - `const LoginLinkTTL = 5 * time.Minute`
  - `func ValidCabinetPath(p string) bool`
  - `func (s *AuthService) CreateLoginLink(ctx, userID int64, next string) (string, error)` → absolute URL `<PublicURL>/login/link?t=<token>`
  - `func (s *AuthService) CabinetLink(ctx, channelType, externalID string) (User, string, error)` — for the bot: user of a linked messenger account; `ErrNotFound` when unknown or not active
  - `func (s *AuthService) UseLoginLink(ctx, token, userAgent string, actor Actor) (User, string /*session token*/, string /*next*/, error)` — `ErrTokenInvalid` for unknown/used/expired/inactive user
  - `func (s *AuthService) CleanLoginLinks(ctx) (int, error)`

- [ ] **Step 1: Migration** — table `login_links (id INTEGER PRIMARY KEY, token_hash BLOB NOT NULL UNIQUE, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, next TEXT NOT NULL, expires_at TEXT NOT NULL, used_at TEXT, created_at TEXT NOT NULL) STRICT`, index on `expires_at`.
- [ ] **Step 2: Failing repo test** `TestLoginLinkMarkUsedOnce`: create, `GetByHash` returns it with `Next`, first `MarkUsed` true, second false; `DeleteExpiredBefore(t0+6m)` removes it.
- [ ] **Step 3: Implement `sqlite.LoginLinks`** (mirror `RegTokens`; `MarkUsed` uses `WHERE used_at IS NULL`), wire in `NewRepos`. Run `go test ./internal/storage/...` → PASS.
- [ ] **Step 4: Failing domain tests** in `loginlink_test.go`:
  - `TestLoginLinkSingleUse`: `CreateLoginLink(u, "/my/feed/3")` → URL prefix `https://example.com/login/link?t=`; `UseLoginLink` → user u, non-empty session, next `/my/feed/3`, `auditCount("auth.login")==1` with details `{"via":"bot_link"}`; second use → `ErrTokenInvalid`.
  - `TestLoginLinkExpires`: advance 5m → `ErrTokenInvalid`; advance 4m59s on a fresh link → OK.
  - `TestLoginLinkNextValidation`: table from Review Focus 3, rejected → `*ValidationError{"next":"invalid"}`.
  - `TestLoginLinkUserDisabledAfterMint`: set status disabled → `ErrTokenInvalid`, no session.
  - `TestLoginLinkRateLimitedPerUser`: 10 links OK, 11th `ErrRateLimited`; another user still OK.
  - `TestCabinetLink`: linked active user → URL with next `/my`; unknown account / pending user → `ErrNotFound`.
- [ ] **Step 5: Implement.** Refactor `VerifyCode`'s session creation into `openSession(ctx, u User, userAgent string, actor Actor, details map[string]any) (string, error)`; link methods reuse it. Limits: `loginlink:<userID>` 10/hour; redeem `linkip:<ip>` 20/15 min. Raw token = 32 random bytes base64url, stored as `HashSecret`. `ValidCabinetPath`: `p == "/my"` or prefix `/my/` or `/my?`, no `//` after the first char, no `\`, no ASCII control chars or spaces.
- [ ] **Step 6:** `go test -race ./internal/domain/... ./internal/storage/...` → PASS. Commit.

### Task 2: HTTP — redeem endpoint, my home, manifest

**Files:**
- Create: `internal/httpapi/cabinet.go`, `internal/httpapi/cabinet_test.go`
- Modify: `internal/httpapi/apitest_test.go` (call `RegisterCabinet`), `cmd/homelander/main.go` (register, cleanup)

**Interfaces:**
- Consumes: Task 1 `UseLoginLink`, `CreateLoginLink`.
- Produces:
  - `func RegisterCabinet(s *Server, api API)` registering `POST /api/v1/auth/link`, `GET /api/v1/me/home` and mounting `/site.webmanifest`.
  - `POST /api/v1/auth/link {token}` → 200 `{"user": Me, "next": "/my..."}` + session cookie; 422 `token_invalid`; 429.
  - `GET /api/v1/me/home` (RequireUser) → `{"telegramLinked": bool, "homes": [{"id", "houseName", "address", "floors": int|null, "apartment", "statusNames": LocalizedText}]}`.
  - `GET /site.webmanifest` → `application/manifest+json`.
  - `func shortName(name string) string`.

- [ ] **Step 1: Failing tests** in `cabinet_test.go`:
  - `TestLoginLinkRedeem`: mint via `e.api.Auth.CreateLoginLink`; `GET /login/link?t=…` (SPA path, no static → 503 is fine) does not consume; POST → 200, cookie set, `next`; `/me` with cookie → 200; second POST → 422 `token_invalid`.
  - `TestLoginLinkSwitchesUser` (Review Focus 2).
  - `TestMyHome`: resident with a residency in a house with address + floors and a Telegram channel → fields; guest → 401.
  - `TestManifest`: site name `{uk:"Тестова громада", en:"Test Community"}`, default locale uk → `name` "Тестова громада", `short_name` "Тестова", `start_url` "/my", `scope` "/", `display` "standalone", `theme_color` "#0ca678", icons 192/512, `Cache-Control: public, max-age=300`; empty name → "Homelander".
  - `TestShortName`: "Homelander"→"Homelander", "Green Garden Society"→"Green", "Супердовгоназвана громада"→first 12 runes trimmed.
- [ ] **Step 2: Implement** `cabinet.go`. Manifest name: `CommunityName[site.DefaultLocale]`, else `cfg.DefaultLocale`, else any value, else "Homelander". Remove `web/public/site.webmanifest`.
- [ ] **Step 3:** wire in `main.go` (`RegisterCabinet`; `auth.CleanLoginLinks` in `cleanup`). `go test -race ./...` → PASS. Commit.

### Task 3: Bot `/cabinet`

**Files:**
- Modify: `internal/messenger/telegram/telegram.go`, `telegram_test.go`, `locales/en.json`, `locales/uk.json`

**Interfaces:**
- Consumes: Task 1 `CabinetLink`.
- Produces: `Activator` gains `CabinetLink(ctx, typ, externalID string) (domain.User, string, error)`; locale keys `bot.commands.cabinet`, `bot.cabinet.offer`, `bot.cabinet.button`.

- [ ] **Step 1: Failing tests:** `TestCabinetSendsLoginButton` (https URL → inline button with `bot.cabinet.button`, text `bot.cabinet.offer` in the user's locale); `TestCabinetLinkAsTextForHTTP`; `TestCabinetUnknownGetsStart` (ErrNotFound → same reply as `/start`); `TestCabinetRateLimited`; `TestSetCommands` expects `start` and `cabinet`.
- [ ] **Step 2: Implement** `case m.Text == "/cabinet" || strings.HasPrefix(m.Text, "/cabinet ")` (also `/cabinet@bot`), share the https-or-text logic with `offerApplication` (`offerLink(chatID, text, button, url)`). Mention `/cabinet` in `bot.help` and `bot.alreadyConnected`.
- [ ] **Step 3:** `go test -race ./internal/messenger/...` → PASS. Commit.

### Task 4: Web — cabinet shell, My home, login redirects

**Files:**
- Create: `web/src/cabinet/sections.ts` (nav registry), `web/src/cabinet/homeCards.tsx` (home card registry), `web/src/pages/cabinet/CabinetLayout.tsx`, `CabinetHome.tsx`, `MyHomeCard.tsx`, `ProfileCard.tsx`, `web/src/pages/LoginLinkPage.tsx`, `web/src/api/cabinet.ts`, tests `web/src/pages/cabinet/Cabinet.test.tsx`, `web/src/pages/LoginLinkPage.test.tsx`
- Modify: `App.tsx`, `LoginPage.tsx` (+test), `SiteHeader.tsx` (+test), `AdminLayout.tsx` (menu entry), `AuditTable.tsx` (via bot link), `lib/reservedPages.ts` + `internal/domain/page.go` (`my`), `lib/format.ts` (`maskPhone`), `api/types.ts`, locales, `test/fixtures.ts`

**Interfaces:**
- Produces:
  - `interface CabinetSection { key: string; to: string; icon: Icon; labelKey: string; exact?: boolean }`, `useCabinetSections(): CabinetSection[]` (Part 3 appends enabled module sections).
  - `interface HomeCard { key: string; Component: ComponentType }`, `cabinetHomeCards: HomeCard[]` (Part 3 prepends the feed card).
  - `useMyHome()` → `MyHome { telegramLinked: boolean; homes: Home[] }`; `useLoginLink()` mutation `{ token } → { user: Me; next: string }`.
  - `maskPhone(phone: string): string` — every digit except the last 4 becomes `•`, `+` kept.
- [ ] **Step 1: Failing tests:** guest at `/my/x?y=1` → `/login?next=%2Fmy%2Fx%3Fy%3D1`; resident sees house, address, floors, apartment, localized status, masked phone `+••••••••0002`, Telegram linked; admin without residencies sees the empty text; nav has "My home" and "Site"; LoginPage sends a resident to `/my` and to `next`; header menu "My cabinet" above "Admin panel"; LoginLinkPage POSTs once (StrictMode-safe), lands on `next`; invalid token → `/login?link=expired` with the message and the `/cabinet` hint; reserved list contains `my`.
- [ ] **Step 2: Implement.** `CabinetLayout`: `AppShell` header 60 (logo, community name, `Account` + `LanguageSwitcher` exported from `SiteHeader`), navbar 240 `visibleFrom="sm"`, footer bottom bar `hiddenFrom="sm"` (icons + labels, safe-area padding). Home: `SimpleGrid`/`Stack` of `cabinetHomeCards`.
- [ ] **Step 3:** `npm test && npm run lint && npm run typecheck` → PASS. Commit.

### Task 5: PWA — service worker, offline page, install card, iOS meta

**Files:**
- Create: `web/public/sw.js`, `web/public/offline.html`, `web/src/lib/pwa.ts`, `web/src/pages/cabinet/InstallCard.tsx`, tests `web/src/lib/sw.test.ts`, `web/src/pages/cabinet/InstallCard.test.tsx`
- Modify: `web/index.html`, `web/src/main.tsx`, `web/src/cabinet/homeCards.tsx`

**Interfaces:**
- Produces: `initPwa()` (captures `beforeinstallprompt`, registers `/sw.js` in production), `useInstall(): { mode: 'prompt' | 'ios' | null; install(): Promise<void>; dismiss(): void }`.
- [ ] **Step 1: Failing tests:** `sw.test.ts` evaluates `sw.js?raw` with a fake `self`/`caches`: install caches `/offline.html`; a navigation to `/my` that fails → offline page; `/api/v1/me`, `/site/x`, `/mcp`, `/oauth/authorize` navigations and any non-navigation request are not handled (`respondWith` not called). InstallCard: with a captured prompt shows Install and calls `prompt()`; iOS UA shows the Share instruction; standalone → hidden; dismiss → hidden and remembered.
- [ ] **Step 2: Implement.** SW: `install` → cache `offline-v1` with `/offline.html`, `skipWaiting`; `activate` → delete other caches, `clients.claim`; `fetch` → only `request.mode === 'navigate'` outside excluded prefixes: `fetch(request).catch(() => caches.match('/offline.html'))`. A comment marks where `push`/`notificationclick` go. index.html: `apple-mobile-web-app-capable`, `mobile-web-app-capable`, `apple-mobile-web-app-title`, `apple-mobile-web-app-status-bar-style=default`, theme-color `#0ca678`.
- [ ] **Step 3:** web checks PASS. Commit.

### Task 6: Verify and ship

- [ ] `make test`, `make lint` green.
- [ ] Run the app: `/my` guard + redirect, manifest JSON, SW not handling `/api/`, login link via curl (GET does not consume, POST once, expiry).
- [ ] Update `docs/development.md` (cabinet, login links, PWA notes). Push, open PR, watch CI.
