# M0 + M1: Foundation and secure resident identity — design

- **Date:** 2026-11-27
- **Status:** Approved by the owner (2026-11-27)
- **Owner:** Igor Artasevych
- **Tracks:** epics #1 (M0 Foundation) and #2 (M1 Secure admin and resident identity)

## 1. Intent

Homelander is a bot and a web system for **one resident community**: the people of one or several
houses who share common chats (a Telegram group split into forum topics today, maybe Discord later).
Over time the system will post automatic information into those chats (power outage schedules
first), show residents a page with the state of their houses (power, batteries, gates), identify
residents, control the yard gate, and run simple user-built automations. Features beyond the core
come as optional **modules** (WASM, see M3).

Before any of that, the system has to know **who is a resident**, with a **verified phone number and
a verified messenger account**, and has to let a few trusted people manage it **safely**. That is
M0 + M1:

- M0 lays the groundwork every later milestone builds on: one Go binary, storage, configuration,
  i18n, the web app shell, CI.
- M1 delivers registration applications, approval, activation through the bot, login with a second
  factor, users, houses and apartments, and a first resident page.

The product is not tied to any particular house. The owner will run the first deployment for their
own community (two houses), but nothing about that community goes into the repository.

## 2. Goals, non-goals, anti-goals

### Goals (M0 + M1)

1. **One public registration link** (`/register`) that can be pinned in the community chat. A person
   enters phone, first name, last name, picks house, apartment and residency status, and submits an
   **application**.
2. An admin **reviews** applications: corrects details if needed, approves or rejects.
3. An admin can **add a user directly** (first name, last name, phone; Resident and Admin flags) —
   for admins and other non-resident people. No application needed.
4. **Every account is activated only through the bot**: the person opens the bot and shares their
   own phone number. Telegram proves the number is theirs; the system links the Telegram account and
   the user becomes active. This link is where second-factor codes go.
5. **Login** for every active user: phone number + one-time code delivered to the linked Telegram
   account. No passwords.
6. Two web areas behind the same login:
   - `/` — the **resident page**: a welcome with the community name and a picture of the houses
     (configured in the admin panel). Later milestones add house data here.
   - `/admin` — the **admin panel**, only for users with the Admin flag.
7. **Everything is configured in the admin panel**, nothing about a community is in code:
   - **site settings**: community name, welcome text, picture, default language;
   - **houses**, each created separately with its own settings: name, real address (later used for
     things like the weather forecast), number of floors, apartment number range;
   - a **house page** with statistics: how many apartments have registered residents, how many
     people live there, by residency status, and an apartment grid showing who is registered where;
   - the **residency status dictionary** (owner / tenant);
   - **users**: searchable list and a user card page with Resident and Admin flags.
8. The **first admin** is defined by environment configuration and activates through the bot.
9. Every user-facing text, in the bot and on the web, goes through **i18n**; Ukrainian is the
   default, English is the second locale.
10. The bot receives Telegram updates by **long polling by default**, switchable to a **webhook** by
    configuration (needed for Railway).
11. Security-relevant actions are written to an **audit log**.

### Non-goals (later milestones)

- House data on the resident page (power, batteries, gates): comes with modules (M3+).
- Registration started from the bot, marking unidentified chat members, chats/topics routing (M2).
- Reacting to group messages. In M1 the bot already receives messages from the community chats it
  is added to and hands them to an event sink that does nothing; it never replies in a group. How it
  reacts is configured later as flows in the visual editor (M5) running WASM modules (M3). Modules
  are written in **Go** (owner, 2026-11-27): TinyGo or standard Go compiled for `wasip1`.
- Modules, module catalog, module settings (M3), outage schedules (M4), flow editor (M5), gate (M6).
- Fine-grained roles / RBAC. M1 has exactly two flags on a user: Resident and Admin.
- Discord or any messenger other than Telegram for activation — but see the messenger abstraction
  (section 5.3).
- Entrances/sections of a house. A big house with sections is entered as several houses
  ("7, section 1", "7, section 2").
- Notifying a person that their application was approved: they are not linked yet, so the bot
  cannot write to them. The page after submitting tells them what to do.
- Multi-tenancy: one deployment serves one community.

### Anti-goals (would be a failure even if it works)

- A secret, a real phone number, address, apartment or picture of a real house in the public
  repository (code, fixtures, issues, docs). Community name, welcome text and picture are settings.
- Any way to get a session without possession of the linked Telegram account.
- An application that gives anything before an admin approved it **and** the person activated it.
- Identity tied to Telegram so tightly that adding another messenger means reworking users.
- Hard-coded Ukrainian (or any) strings outside locale files.

## 3. Constraints

- **Public repository.** All configuration that differs per deployment, and every secret, comes from
  environment variables or from settings stored in the database. `.env.example` lists variables with
  placeholder values only. `.env` is ignored.
- **Pure Go, no CGO.** SQLite via `modernc.org/sqlite`, the same driver as the owner's other projects.
  One static binary.
- **Storage behind an interface.** SQLite is the main and, for now, only implementation; the domain
  never imports the driver.
- **First deployment target: Railway.** One service, one replica, a volume for the SQLite file,
  public HTTPS domain available. The same binary must run at home behind NAT (polling; the bot needs
  no public listener).
- **Telegram facts that shape the design:**
  - A bot cannot find a user by phone number and cannot write first to someone who never started it.
    So activation is started by the person, in the bot.
  - Phone ownership on Telegram is proven by the user tapping **"Share contact"** (`request_contact`
    keyboard button). The bot must check that the shared contact's `user_id` equals the sender's
    id, otherwise someone could forward another person's contact.
  - Polling and webhook are mutually exclusive per bot token; switching mode must call
    `deleteWebhook` / `setWebhook`.
- **Documentation in English**; conversation with the owner in Ukrainian/Russian.
- **Repository workflow:** `main` is protected (PR only, squash merge). Work lands through PRs
  merged by the owner.

## 4. Approaches considered

### Where identity starts

**A. Bot first.** The person starts in the bot, which collects the data and links Telegram at once.
*Trade-off:* fewest steps, but identity begins inside Telegram; another messenger would need a
parallel sign-up.

**B. Web first, messenger linked as a channel. — Chosen by the owner.**
The person applies on the web; the account lives in the system; Telegram is a linked channel used
for activation and codes.
*Trade-off:* one extra hop; in return identity is messenger-agnostic.

### How a resident reaches the application form

**One shared link — chosen by the owner.** `/register` is public and pinned in the community chat;
admins send nothing to anyone. Safe because an application gives nothing until an admin approves it
and the person proves the phone in the bot. Spam is held back by rate limits and form validation.
**Per-person invitation links** (phone prefilled) were rejected: more protection against strangers,
but an admin would have to send a link to every resident.

### Process shape

**One binary** (Claude's call): HTTP API, web app (embedded static build) and the bot worker run in
one Go process, separated internally into packages. Separate services were rejected: one replica on
Railway, SQLite in a file, and a small community make distribution pure cost.

### Web framework

**React + TypeScript** (Claude's call, the owner left it open). The flow editor planned for M5 will
use xyflow, which exists for React (React Flow) and Svelte (Svelte Flow); React Flow is the more
mature of the two, and tables and forms have far more ready-made components in React.

## 5. Design

### 5.1 Repository layout (proposal; builder may adjust names)

```
cmd/homelander/        main: wires config, storage, HTTP server, bot worker
internal/config/       env parsing and validation
internal/domain/       entities and service logic: users, houses, applications, auth, settings
internal/storage/      repository interfaces
internal/storage/sqlite/  implementation + embedded migrations
internal/messenger/    Messenger interface, channel types
internal/messenger/telegram/  Telegram implementation (polling + webhook)
internal/httpapi/      REST API, sessions, middleware, static web serving
internal/i18n/         backend localization
locales/               uk.json, en.json for backend/bot texts
web/                   React app (Vite); builds into web/dist, embedded via go:embed
docs/                  English docs; ADRs in docs/adr/, specs in docs/superpowers/specs/
```

### 5.2 Data model

All ids are integers (SQLite rowid); timestamps are UTC.

| Table | Key fields | Notes |
|---|---|---|
| `users` | id, phone (E.164, unique), first_name, last_name, locale, is_resident, is_admin, status, source (`application` / `admin` / `bootstrap`), approved_by, approved_at, activated_at, created_at | A user is a person. Flags are independent: an admin need not be a resident; turning Resident off removes resident abilities (residencies are kept but ignored). |
| `channels` | id, user_id, type (`telegram`), external_id (Telegram user id), chat_id, username, linked_at | One row per linked messenger account. Unique (type, external_id). At most one channel per type per user in M1. |
| `houses` | id, name (free text, e.g. "22" or "7, section 1"), address (free text), latitude, longitude (optional), floors, apartment_from, apartment_to, sort_order, archived | Apartments are the numbers apartment_from..apartment_to (houses of one complex often continue each other's numbering, e.g. 1–120 and 121–240). Coordinates are entered by the admin (or filled from the address later); modules such as weather will read them. |
| `residency_statuses` | id, code, names (JSON: `{"uk": "Власник", "en": "Owner"}`), sort_order, archived | Dictionary edited in the admin panel; a missing locale falls back to the default locale's name. Seeded with `owner` / `tenant`. |
| `residencies` | id, user_id, house_id, apartment_number, status_id, created_at | One person may have several apartments; **one apartment may have any number of residents** — a second person applying for an already registered apartment is normal. Unique (user_id, house_id, apartment_number). |
| `login_codes` | id, user_id, code_hash, expires_at, attempts, used_at, ip | One active code per user; a new request invalidates the old one. |
| `sessions` | id, user_id, token_hash, created_at, last_seen_at, expires_at, ip, user_agent | Server-side sessions; the cookie holds the raw token. |
| `audit_log` | id, at, actor_user_id, action, target, details (JSON), ip | Append-only. |
| `settings` | key, value (JSON) | Community name and welcome text (per locale), default locale. Secrets never go here. |
| `assets` | id, kind (`hero`), content_type, data (BLOB), sha256, uploaded_by, uploaded_at | Uploaded site picture. In the database so it is backed up with everything else. |

**User status:**

```
application ──▶ pending_approval ──approve──▶ awaiting_activation ──bot: phone shared──▶ active
                       │                              ▲
                       └──reject──▶ rejected          │
admin adds user / bootstrap ──────────────────────────┘
active ◀──▶ disabled (admin)
```

Migrations: SQL files embedded in the binary, applied at start, forward-only, versions in a
`schema_migrations` table (a small own migrator as in the owner's Ganglion project, or `goose` —
builder's call).

The domain talks to storage through repository interfaces (`UserRepo`, `HouseRepo`, …) plus a
transaction runner, so a second database can be added without touching the domain.

### 5.3 Messenger abstraction

```go
type Messenger interface {
    Type() string                                   // "telegram"
    SendText(ctx, chatID string, text string) error // codes, notices
    BotURL() string                                 // public link to the bot (t.me/<bot>)
}
```

Inbound events (`/start`, a contact shared) are turned by the Telegram adapter into
messenger-neutral domain calls (`ActivateChannel(type, externalID, chatID, verifiedPhone)`). The
domain never sees a Telegram type. Discord later implements the same interface with its own proof.

**Telegram adapter.** Library: `gotgbot/v2` for Bot API calls (no dependencies, generated from the
Bot API spec; the owner's Ganglion research rated it the best fit). Receiving:

- `TELEGRAM_UPDATES=polling` (default): long polling.
- `TELEGRAM_UPDATES=webhook`: the HTTP server exposes `POST /telegram/webhook/<random path>`;
  at start the bot calls `setWebhook(PUBLIC_URL + path, secret_token=TELEGRAM_WEBHOOK_SECRET)`;
  the handler compares the `X-Telegram-Bot-Api-Secret-Token` header in constant time.
- Switching back to polling calls `deleteWebhook` at start.
- Library updater or a thin own loop is the builder's call; losing a `/start` on a crash is
  acceptable in M1 (the person just taps again).
- The bot only talks in private chats in M1; it ignores group messages (group work is M2).

### 5.4 Flows

**Apply (resident, public web, `/register`).**
1. Form: phone, first name, last name, house (select), apartment (select from that house's range),
   residency status (select), consent checkbox for storing the data. Captcha is not used.
2. Submit → validation (phone normalized to E.164 with `DEFAULT_PHONE_REGION`, `UA`; apartment exists
   in the house) → a `pending_approval` user with `is_resident = true` and the residency.
3. If the phone already belongs to a user, nothing is created or changed, but the answer is the same
   (no way to probe which numbers are registered). A `rejected` user may apply again: the row is
   reused and goes back to `pending_approval`.
4. Final screen: "Application received. After an administrator approves it, open the bot and share
   your phone number" with the bot button and a QR code for desktop.
5. Rate limits: per IP (e.g. 5 applications / hour) and a global cap on pending applications
   (e.g. 200) after which the form says "try later".

**Review (admin).** The admin panel shows pending applications with a counter in the menu. The admin
may correct name/house/apartment/status, then **approves** (`awaiting_activation`) or **rejects**
(`rejected`).

**Add user (admin).** Form: first name, last name, phone, Resident and Admin flags (if Resident:
residencies can be added on the user card). The user is created as `awaiting_activation`.

**Activate (bot).** Used by everyone, including the first admin.
1. The person opens the bot and sends `/start`. The bot replies with a "Share my phone number"
   keyboard button.
2. Contact received → the bot checks `contact.user_id == from.id` (the person shared **their own**
   number, which Telegram has verified) and normalizes the phone.
3. The domain looks the phone up:
   - `awaiting_activation` → channel linked, user `active`, the bot greets them and gives the web
     link;
   - `active` with this same Telegram account → "you are already connected";
   - `pending_approval` → "your application is waiting for an administrator's approval";
   - anything else (unknown, `rejected`, `disabled`, or `active` with a different Telegram account)
     → one neutral "this number is not registered, contact an administrator".
4. Rate limit per Telegram account (e.g. 5 attempts / hour).

**Bootstrap.** If `BOOTSTRAP_ADMIN_PHONE` is set and no admin exists, a contact shared from that
phone creates an `active` user with `is_admin = true`, `is_resident = false`, `source = bootstrap`,
links the channel and asks them to fill in their name on the web. No tokens, nothing printed to
logs, restarts are harmless. Once an admin exists, the variable is ignored.

**Login (web, any active user).**
1. Phone → `POST /api/v1/auth/code`. The response is always the same ("if this number has access, a
   code was sent"). A code is sent only if the user is `active` and has a channel.
2. Code: 6 digits, stored as HMAC-SHA256 with `APP_SECRET`, lifetime 5 minutes, max 5 attempts,
   one active code per user. The Telegram message includes the time of the request and "if it
   wasn't you, ignore this".
3. `POST /api/v1/auth/verify` → session created; the code is used up. After login an admin lands on
   `/admin`, everyone else on `/`.
4. Rate limits: per phone (e.g. 3 codes / 15 min) and per IP (e.g. 10 requests / 15 min); an
   in-memory limiter is enough for one replica.

**Sessions.** 32 random bytes in a cookie `__Host-session` (`HttpOnly`, `Secure`, `SameSite=Strict`,
`Path=/`); the database keeps only a hash. Idle timeout 12 h, absolute lifetime 7 days (configurable).
Logout deletes the row. Revoking admin, disabling a user, unlinking their channel or changing their
phone deletes all their sessions. Mutating API calls also require a custom header
(`X-Requested-With`) as a CSRF backstop.

**User card (admin).** A separate page per user: name, phone, status, source, linked channels (with
"unlink", which returns the user to `awaiting_activation`), residencies (add/edit/remove), the
**Resident** and **Admin** checkboxes, disable/enable, recent audit entries for this user. The last
active admin cannot lose admin or be disabled. An admin can edit the phone of a user who is not
active yet (uniqueness enforced); an active user's phone changes only after unlinking.

### 5.5 HTTP surface

- `/api/v1/...` JSON REST.
  - Public: `site` (community name, welcome text, picture URL, bot URL), `register/options`
    (houses with apartment ranges, statuses), `register` (submit), `auth/code`, `auth/verify`,
    `assets/{id}`.
  - Signed in: `me`, `auth/logout`.
  - Admin only: everything else.
- Web routes (one React app): `/` resident page, `/register`, `/login`, `/admin/*`. Unknown non-API
  paths fall back to `index.html`.
- `/healthz` for Railway.
- `/telegram/webhook/<path>` only in webhook mode.
- Security headers: CSP (self only), `X-Content-Type-Options`, `Referrer-Policy: no-referrer`,
  `frame-ancestors 'none'`.
- Uploaded pictures: PNG, JPEG or WebP only (no SVG), up to 5 MB, served with their stored content
  type and `X-Content-Type-Options: nosniff`.

### 5.6 Web app (M1 screens)

**Resident page `/`.** Header with the community name; a large picture of the houses; the welcome
text. The product stays universal through three layers, each optional:

1. **Text** — community name and welcome text per locale (Site settings). Defaults to a generic
   greeting.
2. **Built-in illustration** — drawn by the app from the house list: one stylized house per house
   record, labelled with its name. Works out of the box for any community, no upload needed.
3. **Own picture** — an uploaded image replaces the built-in illustration. A specific community's
   look (for example a 3D drawing of its real building) is just such an upload; the product never
   knows about it.

Richer, data-driven scenes (power, batteries, gates on the picture, as in the owner's K7 project)
come later with modules and are out of M1. A guest sees a "Log in" button and a
"Register" link; a signed-in user sees their name, their apartments and "Log out"; an admin also sees
a link to the admin panel. Placeholders for future house data are not shown.

**Register `/register`.** The application form and the final screen (5.4).

**Login `/login`.** Phone → code.

**Admin `/admin`.** Applications (approve / reject, counter), Users (list with search by first name,
last name, phone; filters by house, Resident/Admin flag, status; "Add user"), User card (separate
page, 5.4), Houses (list and a house page: settings — name, address, coordinates, floors,
apartment range; statistics — apartments with registered residents out of total, number of people,
split by residency status and by user status; an apartment grid where each cell shows how many
people are registered and opens the list of them), Residency statuses (CRUD), Site (community name and
welcome text per locale, picture upload, default language, the public registration link with a copy
button and QR code), Audit log (read-only list). Language switcher (uk/en) everywhere.

Stack: React + TypeScript + Vite, `react-router`, `react-i18next`, TanStack Query for API calls.
Component library is the builder's call (Mantine suggested: complete admin-style components, no
Tailwind setup). Dev mode: the Vite dev server proxies `/api` to the Go server.

The picture of the owner's own houses (an L-shaped building with an arch, drawn from the map) is made
outside the repository and uploaded through Site settings. It is not part of M1's code.

### 5.7 i18n

- Backend/bot: locale files `locales/uk.json`, `locales/en.json` embedded in the binary, loaded by
  `nicksnyder/go-i18n/v2` (plural forms). Bot message language: the user's `locale`, else the
  Telegram `language_code` if supported, else the system default.
- Web: `web/src/locales/{uk,en}.json` with `react-i18next`.
- Content entered by admins (community name, welcome text, status names) is stored per locale.
- Default locale from `DEFAULT_LOCALE` (`uk`), overridable in Site settings.
- A CI check fails if a key exists in one locale and not the other.

### 5.8 Configuration (environment only)

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `APP_SECRET` | yes | — | HMAC key for codes and tokens (≥ 32 bytes) |
| `PUBLIC_URL` | yes | — | Base URL for links and the webhook |
| `HTTP_ADDR` | no | `:8080` (or `:$PORT` on Railway) | Listen address |
| `DATABASE_PATH` | no | `./data/homelander.db` | SQLite file |
| `TELEGRAM_BOT_TOKEN` | yes | — | Bot token |
| `TELEGRAM_UPDATES` | no | `polling` | `polling` or `webhook` |
| `TELEGRAM_WEBHOOK_SECRET` | if webhook | — | Secret token for webhook calls |
| `BOOTSTRAP_ADMIN_PHONE` | no | — | First admin's phone |
| `DEFAULT_LOCALE` | no | `uk` | Default language |
| `DEFAULT_PHONE_REGION` | no | `UA` | Region for phone normalization |
| `SESSION_IDLE_TTL`, `SESSION_MAX_TTL` | no | `12h`, `168h` | Session lifetimes |
| `LOG_LEVEL` | no | `info` | Logging |

The process refuses to start on a missing required variable and never logs secret values.

### 5.9 Deployment

Dockerfile (multi-stage: build web, build Go with the web build embedded, distroless/static runtime).
Railway: one service, a volume mounted for `DATABASE_PATH`, `TELEGRAM_UPDATES=webhook`,
`PUBLIC_URL=https://<railway domain>`. The deployment for the owner's community is configured in
Railway only; its name, addresses and data never enter the repository. Database backups are out of
scope for M1 (a follow-up issue).

### 5.10 Testing

- Domain services: unit tests with a temporary SQLite database.
- Status machine: every allowed transition, and every forbidden one refused.
- Applications: duplicate phone answered the same and changes nothing; rejected user can reapply;
  apartment outside the house refused; rate limits.
- Activation: a forwarded contact (contact.user_id ≠ from.id) refused; each status gets the right
  answer; bootstrap only while no admin exists.
- Auth: code expiry, attempt limit, one active code, rate limits, session idle/absolute expiry,
  sessions ended on admin removal / disable / unlink / phone change, identical responses.
- Flags: Resident and Admin independent; last admin protected; admin-only API refuses residents.
- Telegram adapter behind an interface; a fake messenger records sent messages for tests.
- HTTP API: handler tests with `httptest`. Upload: SVG and oversized files refused.
- Web: type check + lint in CI; component tests for the login and application forms.
- CI (GitHub Actions): `go vet`, `golangci-lint`, `go test -race`, web lint/typecheck/test/build,
  locale key parity, Docker build.

## 6. Left to the builder

Exact package names, migration tool (own vs `goose`), Telegram receive loop (library updater vs own),
component library, HTTP router (`net/http` mux is enough), logging (`log/slog` suggested), exact
rate-limit numbers within the stated spirit, QR code library, the style of the built-in illustration,
test fixtures (synthetic data only).

## 7. Calls made by Claude (for the owner to check)

- One binary; React + TS; `gotgbot/v2`; `go-i18n`; Mantine suggested.
- A person can have several apartments; an apartment can have several residents.
- Apartments as a number range per house (not 1..N), so neighbouring houses can continue numbering; letters like "12a" are not supported in M1.
- House coordinates entered manually in M1; geocoding from the address later.
- Floors are stored as information only in M1 (no apartment-to-floor mapping).
- Login is open to every active user; `/admin` only to admins.
- Bootstrap: the first contact shared from `BOOTSTRAP_ADMIN_PHONE` creates the admin (not a resident).
- Applications from an already registered phone get the same answer and change nothing; rejected
  people may apply again.
- The bot gives one neutral answer for unknown / rejected / disabled / conflicting phones.
- Unlinking Telegram sends the user back to "awaiting activation".
- The site picture is stored in the database; SVG uploads are refused.
- A consent checkbox on the application form.
- Session lifetimes 12 h idle / 7 days absolute; code 6 digits, 5 minutes, 5 attempts.
- English as the second locale.
