# M0 + M1: Foundation and secure resident identity — design

- **Date:** 2026-11-27
- **Status:** Draft, awaiting owner approval
- **Owner:** Igor Artasevych
- **Tracks:** epics #1 (M0 Foundation) and #2 (M1 Secure admin and resident identity)

## 1. Intent

Homelander is a bot and a closed admin system for **one resident community**: the people of one or
several houses who share common chats (a Telegram group split into forum topics today, maybe Discord
later). Over time the system will post automatic information into those chats (power outage
schedules first), identify residents, control the yard gate, and run simple user-built automations.
Features beyond the core come as optional **modules** (WASM, see M3).

Before any of that, the system has to know **who is a resident** and has to let a small number of
trusted people manage it **safely**. That is M0 + M1:

- M0 lays the groundwork every later milestone builds on: one Go binary, storage, configuration,
  i18n, the admin UI shell, CI.
- M1 delivers secure login, invitations, residents, houses and apartments.

The product is not tied to any particular house. The owner will run the first deployment for their
own community (two houses), but nothing about that community goes into the repository.

## 2. Goals, non-goals, anti-goals

### Goals (M0 + M1)

1. An admin logs in to the admin panel with a **phone number + one-time code delivered to their
   linked Telegram account**. There is no password and no open sign-up.
2. An admin **invites** a person by phone number. The person opens the invitation page, enters first
   and last name, picks house, apartment and residency status from lists, and links their Telegram
   account. They become a **resident**.
3. An admin manages **houses** (name, number of apartments), the **residency status dictionary**
   (e.g. owner / tenant), **residents** (view, edit, disable) and **admin rights** (grant / revoke).
4. The **first admin** is created from environment configuration.
5. Every user-facing text, in the bot and in the UI, goes through **i18n**; Ukrainian is the
   default, English is the second locale.
6. The bot receives Telegram updates by **long polling by default**, switchable to a **webhook** by
   configuration (needed for Railway).
7. Security-relevant actions are written to an **audit log**.

### Non-goals (later milestones)

- Registration started from the bot, marking unidentified chat members, chats/topics routing (M2).
- Modules, module catalog, module settings (M3), outage schedules (M4), flow editor (M5), gate (M6).
- Fine-grained roles / RBAC. M1 has exactly two levels: resident and admin.
- A panel for regular residents. Residents use the bot only; the only web page they see is the
  invitation page.
- Discord or any messenger other than Telegram — but see the messenger abstraction (section 5.3).
- Entrances/sections of a house. A big house with sections is entered as several houses
  ("7, section 1", "7, section 2").
- Multi-tenancy: one deployment serves one community.

### Anti-goals (would be a failure even if it works)

- A secret, a real phone number, address or apartment in the public repository (code, fixtures,
  issues, docs).
- Any way to get an admin session without possession of the linked Telegram account.
- Identity tied to Telegram so tightly that adding another messenger means reworking users.
- Hard-coded Ukrainian (or any) strings outside locale files.

## 3. Constraints

- **Public repository.** All configuration that differs per deployment, and every secret, comes from
  environment variables. `.env.example` lists them with placeholder values only. `.env` is ignored.
- **Pure Go, no CGO.** SQLite via `modernc.org/sqlite`, the same driver as the owner's Ganglion
  project. One static binary.
- **Storage behind an interface.** SQLite is the main and, for now, only implementation; the domain
  never imports the driver.
- **First deployment target: Railway.** One service, one replica, a volume for the SQLite file,
  public HTTPS domain available. The same binary must run at home behind NAT (polling, no public
  listener needed for the bot).
- **Telegram limits that shape the design:**
  - A bot cannot find a user by phone number and cannot write first to someone who never started it.
    So a code can only be delivered **after** the person linked Telegram, and the **invitation is
    delivered by the admin** (copy a link, send it any way they like).
  - Phone ownership on Telegram is proven by the user tapping **"Share contact"** (`request_contact`
    keyboard button). The bot must check that the shared contact's `user_id` equals the sender's
    id, otherwise someone could forward another person's contact.
  - Polling and webhook are mutually exclusive per bot token; switching mode must call
    `deleteWebhook` / `setWebhook`.
- **Documentation in English**; conversation with the owner in Ukrainian/Russian.
- **Repository workflow:** `main` is protected (PR only, squash merge). Work lands through PRs
  merged by the owner.

## 4. Approaches considered

### Invitation and Telegram linking

**A. Bot first.** The invitation is a deep link `t.me/<bot>?start=<token>`. The bot asks for the
contact, checks the phone against the invitation, then sends a one-time link to a web form for name,
house, apartment, status.
*Trade-off:* one step shorter for the person, but the person's very first step and their identity
are inside Telegram; a Discord-only community would need a parallel flow.

**B. Web first, messenger linked as a channel. — Chosen by the owner.**
The invitation is a web link. The page shows the prefilled phone, the person fills in the form, then
links Telegram through a deep link and "Share contact". The user record lives in the system;
Telegram is one linked channel.
*Trade-off:* one extra hop for the person; in return identity is messenger-agnostic, which is what
the owner asked for, and adding Discord later means adding a channel type, not a second sign-up.

### Process shape

**One binary** (my call): HTTP API, admin UI (embedded static build), invitation page and the bot
worker run in one Go process, separated internally into packages. Separate services were rejected:
one replica on Railway, SQLite in a file, and a tiny community make distribution pure cost.

### Admin UI framework

**React + TypeScript** (my call, the owner left it open). The flow editor planned for M5 will use
xyflow, which exists for React (React Flow) and Svelte (Svelte Flow); React Flow is the more mature
of the two, and the admin panel's tables and forms have far more ready-made components in React.

## 5. Design

### 5.1 Repository layout (proposal; builder may adjust names)

```
cmd/homelander/        main: wires config, storage, HTTP server, bot worker
internal/config/       env parsing and validation
internal/domain/       entities and service logic: users, houses, invitations, auth
internal/storage/      repository interfaces
internal/storage/sqlite/  implementation + embedded migrations
internal/messenger/    Messenger interface, channel types
internal/messenger/telegram/  Telegram implementation (polling + webhook)
internal/httpapi/      REST API, sessions, middleware, static UI serving
internal/i18n/         backend localization
locales/               uk.json, en.json for backend/bot texts
web/                   React app (Vite); builds into web/dist, embedded via go:embed
docs/                  English docs; ADRs in docs/adr/, specs in docs/superpowers/specs/
```

### 5.2 Data model

All ids are integers (SQLite rowid); timestamps are UTC.

| Table | Key fields | Notes |
|---|---|---|
| `users` | id, phone (E.164, unique), first_name, last_name, locale, is_admin, status (`active` / `disabled`), created_at | Every registered person is a resident. `is_admin` is the only right in M1. |
| `channels` | id, user_id, type (`telegram`), external_id (Telegram user id), chat_id, username, linked_at | One row per linked messenger account. Unique (type, external_id). A user may have at most one channel per type in M1. |
| `houses` | id, name (free text, e.g. "22" or "7, section 1"), apartments_count, sort_order, archived | Apartments are numbered 1..apartments_count. |
| `residency_statuses` | id, code, names (JSON: `{"uk": "Власник", "en": "Owner"}`), sort_order, archived | Dictionary edited in the UI, one name per supported locale (a missing locale falls back to the default locale's name). Seeded with `owner` / `tenant`. |
| `residencies` | id, user_id, house_id, apartment_number, status_id, created_at | Many-to-many: one person may live in or own several apartments; one apartment may have several residents. Unique (user_id, house_id, apartment_number). |
| `invitations` | id, phone, token_hash, grant_admin, is_bootstrap, created_by, expires_at, used_at, revoked_at, user_id | One-time. Token is shown once to the admin, only its hash is stored. `grant_admin` makes the person an admin on registration. |
| `login_codes` | id, user_id, code_hash, expires_at, attempts, used_at, ip | One active code per user; a new request invalidates the old one. |
| `link_requests` | id, user_id, token_hash, expires_at, used_at | Short-lived token behind the `t.me/<bot>?start=` deep link used for channel linking. |
| `sessions` | id, user_id, token_hash, created_at, last_seen_at, expires_at, ip, user_agent | Server-side sessions; cookie holds the raw token. |
| `audit_log` | id, at, actor_user_id, action, target, details (JSON), ip | Append-only. |
| `settings` | key, value | System settings editable in the UI (e.g. default locale). Secrets never go here. |

Migrations: SQL files embedded in the binary, applied at start, forward-only, versions in a
`schema_migrations` table (the same approach as Ganglion ADR-0016; a small own migrator or `goose`
— builder's call).

The domain talks to storage through repository interfaces (`UserRepo`, `HouseRepo`, …) plus a
transaction runner, so a second database can be added without touching the domain.

### 5.3 Messenger abstraction

```go
type Messenger interface {
    Type() string                                   // "telegram"
    SendText(ctx, chatID string, text string) error // codes, notices
    LinkURL(token string) string                    // deep link for channel linking
}
```

Inbound events (a contact shared, `/start <token>`) are turned by the Telegram adapter into
messenger-neutral domain calls (`LinkChannel(token, externalID, chatID, phone)`). The domain never
sees a Telegram type. Discord later implements the same interface with its own linking proof.

**Telegram adapter.** Library: `gotgbot/v2` for Bot API calls (no dependencies, generated from the
Bot API spec; the owner's Ganglion research rated it the best fit for "methods only"). Receiving:

- `TELEGRAM_UPDATES=polling` (default): long polling.
- `TELEGRAM_UPDATES=webhook`: the HTTP server exposes `POST /telegram/webhook/<random path>`;
  at start the bot calls `setWebhook(PUBLIC_URL + path, secret_token=TELEGRAM_WEBHOOK_SECRET)`;
  the handler compares the `X-Telegram-Bot-Api-Secret-Token` header in constant time.
- Switching back to polling calls `deleteWebhook` at start.
- Whether to use the library's own updater or a thin own loop is the builder's call; for M1 losing
  a `/start` on a crash is acceptable (the person just taps again).

### 5.4 Flows

**Bootstrap.** If `BOOTSTRAP_ADMIN_PHONE` is set and no admin exists, at start the system creates
an invitation for that phone with `grant_admin` and `is_bootstrap` set and logs the invitation URL
to the server log (at warn level, token included — the only place a raw token is logged). Because
only the hash is stored and the URL cannot be printed again, **on every start while no admin exists**
the previous unused bootstrap invitation is revoked and a fresh one is issued and logged, so a
restart never locks the first admin out. Once an admin exists, the variable is ignored.

**Invite (admin).** Admin enters a phone → system normalizes it to E.164 (default region from
`DEFAULT_PHONE_REGION`, `UA`) → rejects if a user or a live invitation already has it → creates an
invitation (default lifetime 7 days, `INVITE_TTL`; optional "make admin" checkbox → `grant_admin`) → shows the link `PUBLIC_URL/invite/<token>`
with a copy button. The admin can list, revoke and re-issue invitations.

**Register (invitee, web).**
1. `/invite/<token>`: valid → form with phone (read-only), first name, last name, house (select),
   apartment (select 1..N of that house), status (select). Invalid/expired/used → a neutral error page.
2. Submit → user + residency created, invitation marked used, user `active` but **without a channel**.
3. Next screen: "Connect Telegram" button = `t.me/<bot>?start=<link token>` (+ QR code for desktop).
4. In the bot: `/start <link token>` → bot replies with a "Share contact" keyboard button.
5. Contact received → checks: `contact.user_id == from.id`; normalized contact phone equals the
   user's phone; link token valid. Pass → channel created, bot confirms in the user's language.
   Fail → bot explains (in neutral words) and nothing is linked.
6. The web page polls the link status and shows "Done".

The "Connect Telegram" step lives on its own page `/link/<link token>` (button, QR code, status
polling), so it can be reopened. Link tokens live 24 h. The registration form itself is not
reachable again once the invitation is used.

A registered user without a channel can get a new link from an admin ("send link again"): the
system creates a new link token and the admin copies the `PUBLIC_URL/link/<token>` web URL, the same
way as an invitation.

**Phone mismatch.** If the person's Telegram phone differs from the invited one, linking fails and
the bot tells them to contact an admin. An admin can **edit the phone of a user who has no linked
channel** (uniqueness still enforced), then resend the link. Changing the phone of a linked user
requires unlinking the channel first (which also ends their sessions).

**Login (admin, web).**
1. Phone → `POST /api/v1/auth/code`. The response is always the same ("if this number has access, a
   code was sent"). A code is sent only if the user exists, is active, is admin and has a channel.
2. Code: 6 digits, stored as HMAC-SHA256 with `APP_SECRET`, lifetime 5 minutes, max 5 attempts,
   one active code per user. The Telegram message includes the IP/time of the request and "if it
   wasn't you, ignore this".
3. `POST /api/v1/auth/verify` → session created; old code invalidated.
4. Rate limits: per phone (e.g. 3 codes / 15 min) and per IP (e.g. 10 requests / 15 min); in-memory
   limiter is enough for one replica.

**Sessions.** 32 random bytes in a cookie `__Host-session` (`HttpOnly`, `Secure`, `SameSite=Strict`,
`Path=/`); the database keeps only a hash. Idle timeout 12 h, absolute lifetime 7 days (configurable).
Logout deletes the row. Revoking admin, disabling a user or unlinking their channel deletes all their
sessions. Mutating API calls also require a custom header (`X-Requested-With`) as a CSRF backstop.

**Admin rights.** An admin can grant or revoke `is_admin` on any active user; the last admin cannot
revoke themselves or be disabled.

### 5.5 HTTP surface

- `/api/v1/...` JSON REST. Public: `auth/code`, `auth/verify`, `invitations/{token}` (read + submit),
  `link/{token}` (read + status). Everything else requires an admin session.
- `/invite/*`, `/link/*` and `/` — the React app (one SPA with a public invitation route and the protected admin
  area). Unknown non-API paths fall back to `index.html`.
- `/healthz` for Railway.
- `/telegram/webhook/<path>` only in webhook mode.
- Security headers: CSP (self only), `X-Content-Type-Options`, `Referrer-Policy: no-referrer`,
  `frame-ancestors 'none'`.

### 5.6 Admin UI (M1 screens)

Login (phone → code), Residents (list, search, filter by house; details with residencies and
channel; edit, disable, grant/revoke admin, resend link), Invitations (create, list, revoke),
Houses (CRUD, apartments count), Residency statuses (CRUD), Audit log (read-only list),
Settings (default locale). Language switcher (uk/en).

Stack: React + TypeScript + Vite, `react-router`, `react-i18next`, TanStack Query for API calls.
Component library is the builder's call (Mantine is my suggestion: complete admin-style components,
no Tailwind setup). Dev mode: Vite dev server proxies `/api` to the Go server.

### 5.7 i18n

- Backend/bot: locale files `locales/uk.json`, `locales/en.json` embedded in the binary, loaded by
  `nicksnyder/go-i18n/v2` (plural forms). Bot message language: the user's `locale`, else the
  Telegram `language_code` if supported, else the system default.
- UI: `web/src/locales/{uk,en}.json` with `react-i18next`.
- Default locale from `DEFAULT_LOCALE` (`uk`), overridable in Settings.
- A CI check fails if a key exists in one locale and not the other.

### 5.8 Configuration (environment only)

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `APP_SECRET` | yes | — | HMAC key for codes and tokens (≥ 32 bytes) |
| `PUBLIC_URL` | yes | — | Base URL for invitation links and webhook |
| `HTTP_ADDR` | no | `:8080` (or `:$PORT` on Railway) | Listen address |
| `DATABASE_PATH` | no | `./data/homelander.db` | SQLite file |
| `TELEGRAM_BOT_TOKEN` | yes | — | Bot token |
| `TELEGRAM_UPDATES` | no | `polling` | `polling` or `webhook` |
| `TELEGRAM_WEBHOOK_SECRET` | if webhook | — | Secret token for webhook calls |
| `BOOTSTRAP_ADMIN_PHONE` | no | — | First admin's phone |
| `DEFAULT_LOCALE` | no | `uk` | Default language |
| `DEFAULT_PHONE_REGION` | no | `UA` | Region for phone normalization |
| `INVITE_TTL`, `SESSION_IDLE_TTL`, `SESSION_MAX_TTL` | no | `168h`, `12h`, `168h` | Lifetimes |
| `LOG_LEVEL` | no | `info` | Logging |

The process refuses to start on a missing required variable and never logs secret values.

### 5.9 Deployment

Dockerfile (multi-stage: build web, build Go with the web build embedded, distroless/static runtime).
Railway: one service, a volume mounted for `DATABASE_PATH`, `TELEGRAM_UPDATES=webhook`,
`PUBLIC_URL=https://<railway domain>`. The deployment for the owner's community is configured in
Railway only; its name, addresses and data never enter the repository. Database backups are out of
scope for M1 (a follow-up issue).

### 5.10 Testing

- Domain services: unit tests with an in-memory/temporary SQLite database.
- Auth: tests for code expiry, attempt limit, one-active-code, rate limits, session idle/absolute
  expiry, revocation on admin removal, generic responses.
- Linking: contact from a different user rejected; phone mismatch rejected; expired token rejected.
- Telegram adapter behind an interface; a fake messenger records sent messages for tests.
- HTTP API: handler tests with `httptest`.
- UI: type check + lint in CI; a few component tests for the login and invitation forms.
- CI (GitHub Actions): `go vet`, `golangci-lint`, `go test -race`, web lint/typecheck/test/build,
  locale key parity, Docker build.

## 6. Left to the builder

Exact package names, migration tool (own vs `goose`), Telegram receive loop (library updater vs own),
UI component library, HTTP router (`net/http` mux is enough), logging library (`log/slog` suggested),
exact rate-limit numbers within the stated spirit, QR code library, test fixtures (synthetic data only).

## 7. Calls made by Claude (for the owner to check)

- One binary, React + TS admin UI, `gotgbot/v2`, `go-i18n`, Mantine suggested.
- A person can have several residencies; an apartment can have several residents.
- Only admins can log in to the web; non-admin residents get the same neutral response and no code.
- Bootstrap admin via a one-time invitation URL printed to the server log, reissued on each start until an admin exists.
- Admins can issue "make admin" invitations.
- Admins can fix the phone of a not-yet-linked user; a linked user's phone changes only after unlinking.
- Status names stored per locale in the database.
- Epics #1 and #2 have no sub-issues yet; the implementation plan defines them (one issue per PR-sized slice).
- Session lifetimes 12 h idle / 7 days absolute; code 6 digits, 5 minutes, 5 attempts.
- English as the second locale.
