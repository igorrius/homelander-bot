# M0 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One Go binary that loads configuration from the environment, opens and migrates SQLite, localizes texts, serves a health check and an embedded React app shell, ships as a Docker image, and is checked by CI.

**Architecture:** `cmd/homelander` wires `internal/config`, `internal/storage/sqlite`, `internal/i18n` and `internal/httpapi`. The React app in `web/` builds into `web/dist`, embedded by package `web` and served by `httpapi` with an SPA fallback. No domain logic yet: M1 plans 2 (backend and bot) and 3 (web screens) build on this.

**Tech Stack:** Go 1.27, `modernc.org/sqlite` v1.63, `nicksnyder/go-i18n/v2` v2.7, `log/slog`, `net/http`; React 19, TypeScript 5, Vite 8, Mantine 9, react-router 8, react-i18next 18, TanStack Query 5, Vitest 5, ESLint; GitHub Actions; Docker (distroless static).

**Spec:** `docs/superpowers/specs/2026-11-27-m0-m1-foundation-and-identity-design.md` (sections 3, 5.1, 5.5, 5.7, 5.8, 5.9, 5.10).

## Global Constraints

- Module path `github.com/igorrius/homelander-bot`; Go 1.27; **no CGO** (`CGO_ENABLED=0` in every build).
- Configuration and secrets **only** from environment variables; `.env.example` holds placeholders only; never log a secret value.
- Supported locales `uk` (default) and `en`; no user-facing string outside locale files.
- Documentation, code comments, commit messages in English.
- No real addresses, phone numbers or community names anywhere in the repository, including tests (use `+380000000000`-style synthetic numbers).
- Work on a branch, open a PR; `main` accepts squash-merged PRs only.

## Review Focus

1. Several required variables missing at once → start-up error names **all** of them, not just the first (Task 1 test `TestLoadReportsAllMissing`).
2. Logging the configuration → `APP_SECRET`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET` appear as `[redacted]` (Task 1 test `TestConfigLogValueRedactsSecrets`).
3. Browser refresh on a deep link (`/admin/users/5`) → the app's `index.html`; but `/api/v1/nope` → JSON 404 and `/assets/missing.js` → plain 404, never `index.html` (Task 4 tests).
4. `DATABASE_PATH` in a directory that does not exist yet (`./data/…` on a fresh checkout) → directory is created, start succeeds (Task 2 test `TestOpenCreatesDirectory`).
5. A broken migration file → its changes are rolled back, start fails, the next start with a fixed file applies it once (Task 2 test `TestMigrateFailedFileRollsBack`).

---

### Task 1: Go module, configuration and entry point

**Files:**
- Create: `go.mod`, `cmd/homelander/main.go`, `internal/config/config.go`, `internal/config/config_test.go`, `.env.example`
- Modify: `.gitignore` (add `/data/`, `/web/dist/*`, `!/web/dist/.gitkeep`, `/web/node_modules/`, `/homelander`)

**Interfaces:**
- Produces:
  ```go
  package config
  type UpdatesMode string
  const (UpdatesPolling UpdatesMode = "polling"; UpdatesWebhook UpdatesMode = "webhook")
  type Config struct {
      AppSecret             []byte
      PublicURL             *url.URL
      HTTPAddr              string
      DatabasePath          string
      TelegramBotToken      string
      TelegramUpdates       UpdatesMode
      TelegramWebhookSecret string
      BootstrapAdminPhone   string
      DefaultLocale         string
      DefaultPhoneRegion    string
      SessionIdleTTL        time.Duration
      SessionMaxTTL         time.Duration
      LogLevel              slog.Level
  }
  func Load(getenv func(string) string) (Config, error)
  func (c Config) LogValue() slog.Value
  var SupportedLocales = []string{"uk", "en"}
  ```

Rules for `Load` (spec 5.8): required `APP_SECRET` (≥ 32 bytes), `PUBLIC_URL` (absolute, scheme `https`, or `http` only for host `localhost`/`127.0.0.1`), `TELEGRAM_BOT_TOKEN`; `TELEGRAM_WEBHOOK_SECRET` required when `TELEGRAM_UPDATES=webhook`. Defaults: `HTTP_ADDR` → `":"+PORT` if `PORT` set, else `":8080"`; `DATABASE_PATH` `./data/homelander.db`; `TELEGRAM_UPDATES` `polling`; `DEFAULT_LOCALE` `uk` (must be in `SupportedLocales`); `DEFAULT_PHONE_REGION` `UA`; `SESSION_IDLE_TTL` `12h`; `SESSION_MAX_TTL` `168h` (must be ≥ idle); `LOG_LEVEL` `info`. All problems are collected and returned together with `errors.Join`.

- [ ] **Step 1: Write failing tests** in `internal/config/config_test.go`, using a map-backed `getenv`:
  - `TestLoadDefaults`: only the three required vars set → `HTTPAddr == ":8080"`, `DatabasePath == "./data/homelander.db"`, `TelegramUpdates == UpdatesPolling`, `DefaultLocale == "uk"`, `DefaultPhoneRegion == "UA"`, `SessionIdleTTL == 12*time.Hour`, `SessionMaxTTL == 168*time.Hour`, `LogLevel == slog.LevelInfo`.
  - `TestLoadUsesPort`: `PORT=9000`, no `HTTP_ADDR` → `":9000"`; with `HTTP_ADDR=:7000` too → `":7000"`.
  - `TestLoadReportsAllMissing`: empty env → error text contains `APP_SECRET`, `PUBLIC_URL` and `TELEGRAM_BOT_TOKEN`.
  - `TestLoadRejectsShortSecret`: 31-byte secret → error mentions `APP_SECRET`.
  - `TestLoadPublicURL`: `http://example.com` rejected; `http://localhost:8080` and `https://example.com` accepted.
  - `TestLoadWebhookNeedsSecret`: `TELEGRAM_UPDATES=webhook` without secret → error mentions `TELEGRAM_WEBHOOK_SECRET`; `TELEGRAM_UPDATES=push` → error mentions `TELEGRAM_UPDATES`.
  - `TestLoadRejectsUnsupportedLocale`: `DEFAULT_LOCALE=de` → error.
  - `TestLoadRejectsMaxBelowIdle`: idle `2h`, max `1h` → error.
  - `TestConfigLogValueRedactsSecrets`: log `cfg` with `slog.NewJSONHandler` into a buffer; output contains `[redacted]` and does **not** contain the secret, token or webhook secret values; contains `PublicURL`.

- [ ] **Step 2: Run** `go test ./internal/config/` — Expected: FAIL (package does not compile).
- [ ] **Step 3: Implement** `Load` and `LogValue` in `internal/config/config.go` per the rules above.
- [ ] **Step 4: Run** `go test ./internal/config/` — Expected: PASS.
- [ ] **Step 5: Implement `cmd/homelander/main.go`:** `run(ctx context.Context, getenv func(string) string, stderr io.Writer) error`; `main` calls it with `signal.NotifyContext(…, os.Interrupt, syscall.SIGTERM)` and exits 1 on error. For now `run` loads config, builds a JSON `slog` logger at `cfg.LogLevel`, logs `"starting"` with `"config", cfg`, and returns. Later tasks extend `run`.
- [ ] **Step 6: Write `.env.example`** listing every variable of spec 5.8 with placeholder values (`APP_SECRET=change-me-to-32-random-bytes-or-more`, `PUBLIC_URL=http://localhost:8080`, `TELEGRAM_BOT_TOKEN=000000:placeholder`, …) and a one-line comment each.
- [ ] **Step 7: Verify** `go vet ./... && CGO_ENABLED=0 go build ./cmd/homelander && ./homelander; echo $?` — Expected: exit 1, stderr names the three required variables.
- [ ] **Step 8: Commit** `feat: configuration from environment and entry point`

### Task 2: SQLite storage and migrations

**Files:**
- Create: `internal/storage/storage.go`, `internal/storage/sqlite/sqlite.go`, `internal/storage/sqlite/migrate.go`, `internal/storage/sqlite/migrations/0001_settings.sql`, `internal/storage/sqlite/sqlite_test.go`, `internal/storage/sqlite/migrate_test.go`
- Modify: `cmd/homelander/main.go`

**Interfaces:**
- Consumes: `config.Config.DatabasePath`.
- Produces:
  ```go
  package storage
  // TxRunner runs fn in one transaction; fn's error rolls it back.
  type TxRunner interface { InTx(ctx context.Context, fn func(ctx context.Context) error) error }

  package sqlite
  func Open(ctx context.Context, path string) (*sql.DB, error)
  func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error
  var Migrations fs.FS // embedded migrations/*.sql
  type DB struct{ *sql.DB }          // implements storage.TxRunner
  func (d DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error
  func Conn(ctx context.Context, d DB) interface{ ExecContext(...); QueryContext(...); QueryRowContext(...) } // the tx in ctx, else the db
  ```

`Open`: creates the parent directory (`0o750`), DSN `file:<path>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)`, `SetMaxOpenConns(1)` for writes simplicity, pings. `Migrate`: files named `NNNN_name.sql` (4 digits), applied in numeric order, each in its own transaction together with its row in `schema_migrations(version INTEGER PRIMARY KEY, name TEXT, applied_at TEXT)`; a file not matching the pattern fails with its name; forward-only. `0001_settings.sql` creates `settings(key TEXT PRIMARY KEY, value TEXT NOT NULL CHECK (json_valid(value)), updated_at TEXT NOT NULL)`.

- [ ] **Step 1: Write failing tests:**
  - `TestOpenCreatesDirectory`: path `t.TempDir()+"/a/b/db.sqlite"` → no error, file exists.
  - `TestOpenSetsPragmas`: `PRAGMA journal_mode` = `wal`, `PRAGMA foreign_keys` = `1`.
  - `TestMigrateAppliesInOrderOnce`: `fstest.MapFS` with `0002_b.sql` and `0001_a.sql` (each inserts into a log table created by 0001) → rows in order a, b; second `Migrate` call adds nothing.
  - `TestMigrateFailedFileRollsBack`: `0001` ok, `0002` = `CREATE TABLE x(id INT); SELECT * FROM missing;` → error naming `0002`; table `x` does not exist; `schema_migrations` has only 1; after replacing `0002` with a valid file, `Migrate` applies it once.
  - `TestMigrateRejectsBadNames`: `1_a.sql` → error containing `1_a.sql`.
  - `TestInTxRollsBackOnError`: insert into `settings` inside `InTx`, return an error → row absent; with nil → row present.
  - `TestEmbeddedMigrationsApply`: `Migrate(ctx, db, Migrations)` on a fresh db → `settings` table exists.
- [ ] **Step 2: Run** `go test ./internal/storage/...` — Expected: FAIL.
- [ ] **Step 3: Implement** `Open`, `Migrate`, `DB.InTx` (transaction stored in the context under a private key) and `Conn`.
- [ ] **Step 4: Run** `go test -race ./internal/storage/...` — Expected: PASS.
- [ ] **Step 5: Wire into `run`:** open `cfg.DatabasePath`, `Migrate(ctx, db, sqlite.Migrations)`, `defer db.Close()`; a migration error ends `run` with that error.
- [ ] **Step 6: Commit** `feat: SQLite storage with embedded forward-only migrations`

### Task 3: Backend localization

**Files:**
- Create: `internal/i18n/i18n.go`, `internal/i18n/i18n_test.go`, `locales/uk.json`, `locales/en.json`, `locales/embed.go`

**Interfaces:**
- Consumes: `config.SupportedLocales`, `config.Config.DefaultLocale`.
- Produces:
  ```go
  package locales
  //go:embed *.json
  var FS embed.FS

  package i18n
  type Bundle struct{ /* go-i18n bundle + default */ }
  func New(fsys fs.FS, defaultLocale string, supported []string) (*Bundle, error)
  // Pick returns the first supported locale among candidates ("uk", "en-US" → "en"), else the default.
  func (b *Bundle) Pick(candidates ...string) string
  // T renders message id in locale; data fills {{.Name}} templates; count selects plural form when non-nil.
  func (b *Bundle) T(locale, id string, data map[string]any, count any) string
  ```

Locale files use go-i18n's flat JSON form (`"app.name": "Homelander"`, plurals as `{"one": …, "few": …, "many": …, "other": …}`). Seed keys: `app.name`, `common.error.internal`, `test.apartments` (plural, used by tests: uk one/few/many/other "{{.Count}} квартира/квартири/квартир/квартири", en one/other).

- [ ] **Step 1: Write failing tests:**
  - `TestPickFallsBack`: `Pick("de")` → `"uk"`; `Pick("en-GB")` → `"en"`; `Pick("", "en")` → `"en"`.
  - `TestTUkrainianPlurals`: `T("uk","test.apartments",{"Count":1},1)` → `"1 квартира"`; 3 → `"3 квартири"`; 5 → `"5 квартир"`; 21 → `"21 квартира"`.
  - `TestTMissingInLocaleFallsBackToDefault`: an `fstest.MapFS` where `en` lacks a key → English call returns the Ukrainian text.
  - `TestTMissingEverywhereReturnsID`: unknown id → returns the id itself (visible in UI, never panics).
  - `TestLocaleFilesHaveSameKeys`: load `locales.FS`; flatten keys of `uk.json` and `en.json`; sets equal, otherwise list the differences.
- [ ] **Step 2: Run** `go test ./internal/i18n/` — Expected: FAIL.
- [ ] **Step 3: Implement** `New`, `Pick`, `T` on `go-i18n/v2` (`i18n.NewBundle(language.Ukrainian)`, `RegisterUnmarshalFunc("json", json.Unmarshal)`, `LoadMessageFileFS`).
- [ ] **Step 4: Run** `go test ./internal/i18n/` — Expected: PASS.
- [ ] **Step 5: Wire into `run`:** `i18n.New(locales.FS, cfg.DefaultLocale, config.SupportedLocales)`; error ends `run`.
- [ ] **Step 6: Commit** `feat: backend localization with uk default and plural forms`

### Task 4: HTTP server, health check, SPA serving

**Files:**
- Create: `web/embed.go`, `web/dist/.gitkeep`, `internal/httpapi/server.go`, `internal/httpapi/middleware.go`, `internal/httpapi/spa.go`, `internal/httpapi/server_test.go`
- Modify: `cmd/homelander/main.go`

**Interfaces:**
- Consumes: `config.Config`, `*sql.DB` (for the health ping), `*i18n.Bundle`.
- Produces:
  ```go
  package web
  //go:embed all:dist
  var dist embed.FS
  func Dist() fs.FS // fs.Sub(dist, "dist")

  package httpapi
  type Deps struct { DB *sql.DB; I18n *i18n.Bundle; Log *slog.Logger; Static fs.FS }
  func NewHandler(cfg config.Config, d Deps) http.Handler
  // API mux for later plans: routes are added with Handle on this mux under /api/v1/.
  func (s *Server) API() *http.ServeMux
  func WriteJSON(w http.ResponseWriter, status int, v any)
  func WriteError(w http.ResponseWriter, status int, code string) // {"error":{"code":code}}
  ```

Behaviour (spec 5.5): `GET /healthz` → 200 `{"status":"ok"}` after `db.PingContext` with 2 s timeout, 503 `{"status":"unavailable"}` on failure. `/api/` paths not matched → `WriteError(404,"not_found")`. Other paths: an existing file in `Static` is served (files under `/assets/` with `Cache-Control: public, max-age=31536000, immutable`, `index.html` with `no-cache`); a missing path **with a file extension** → 404; any other path → `index.html`. If `Static` has no `index.html` (dev build without the web), non-API paths answer 503 with a plain text hint "web app is not built". Security headers on every response: `Content-Security-Policy: default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. Request logging middleware at debug level (method, path, status, duration); panics recovered → 500 `internal`.

- [ ] **Step 1: Write failing tests** (`httptest`, `fstest.MapFS` with `index.html`, `assets/app.js`):
  - `TestHealthzOK` and `TestHealthzDBDown` (closed db → 503).
  - `TestSecurityHeadersOnEveryResponse`: for `/healthz`, `/`, `/api/v1/nope` the three headers are present with the exact values above.
  - `TestUnknownAPIReturnsJSON404`: `/api/v1/nope` → 404, `Content-Type: application/json`, body `{"error":{"code":"not_found"}}`.
  - `TestSPAFallback`: `/admin/users/5` → 200, body of `index.html`, `Cache-Control: no-cache`.
  - `TestMissingAssetIs404`: `/assets/missing.js` → 404, body is not `index.html`.
  - `TestAssetCached`: `/assets/app.js` → 200 with the immutable cache header.
  - `TestNoWebBuild`: empty `Static` → `/` 503 with the hint; `/healthz` still 200.
  - `TestPanicRecovered`: a route on `API()` that panics → 500 `{"error":{"code":"internal"}}`.
- [ ] **Step 2: Run** `go test ./internal/httpapi/` — Expected: FAIL.
- [ ] **Step 3: Implement** `NewHandler`, middleware and SPA handler.
- [ ] **Step 4: Run** `go test -race ./internal/httpapi/` — Expected: PASS.
- [ ] **Step 5: Wire into `run`:** `http.Server{Addr: cfg.HTTPAddr, Handler: …, ReadHeaderTimeout: 10*time.Second}`; start in a goroutine; on `ctx.Done()` call `Shutdown` with a 15 s timeout; log `"listening"` with the address.
- [ ] **Step 6: Verify manually:** `cp .env.example .env`, `set -a; . ./.env; set +a; go run ./cmd/homelander` then `curl -i localhost:8080/healthz` → 200 `{"status":"ok"}`; Ctrl-C → process exits 0 with a `"stopped"` log line.
- [ ] **Step 7: Commit** `feat: HTTP server with health check, security headers and SPA serving`

### Task 5: Web app shell

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/vite.config.ts`, `web/eslint.config.js`, `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx`, `web/src/i18n.ts`, `web/src/locales/uk.json`, `web/src/locales/en.json`, `web/src/components/LanguageSwitcher.tsx`, `web/src/pages/HomePage.tsx`, `web/src/pages/RegisterPage.tsx`, `web/src/pages/LoginPage.tsx`, `web/src/pages/admin/AdminLayout.tsx`, `web/src/pages/NotFoundPage.tsx`, `web/src/api/client.ts`, `web/src/test/setup.ts`, `web/src/App.test.tsx`, `web/src/locales/locales.test.ts`

**Interfaces:**
- Consumes: the HTTP behaviour of Task 4 (`/api/v1/*` JSON, SPA fallback).
- Produces (for plan 3):
  ```ts
  // web/src/api/client.ts
  export class ApiError extends Error { status: number; code: string }
  export async function api<T>(path: string, init?: RequestInit): Promise<T> // prefixes /api/v1, sends X-Requested-With: homelander, credentials 'same-origin', throws ApiError
  // web/src/i18n.ts
  export const supportedLocales = ['uk', 'en'] as const
  export function initI18n(): Promise<typeof i18next> // default 'uk', stored choice in localStorage 'locale'
  ```
  Routes in `App.tsx`: `/` HomePage, `/register` RegisterPage, `/login` LoginPage, `/admin/*` AdminLayout (Mantine `AppShell` with an empty nav), `*` NotFoundPage. Each placeholder page shows a localized title only.

Scripts in `package.json`: `dev` (vite), `build` (`tsc -b && vite build`, outDir `dist`, `emptyOutDir: true` but keep `.gitkeep` by writing it in a `closeBundle` hook or by `git checkout` — implementer's choice), `lint` (`eslint .`), `typecheck` (`tsc -b --noEmit`), `test` (`vitest run`). Vite dev server proxies `/api` and `/healthz` to `http://localhost:8080`.

- [ ] **Step 1: Scaffold** with `npm create vite@latest web -- --template react-ts`, then add `@mantine/core @mantine/hooks react-router react-i18next i18next @tanstack/react-query` and dev deps `vitest @testing-library/react @testing-library/jest-dom jsdom`. Remove template demo files.
- [ ] **Step 2: Write failing tests:**
  - `locales.test.ts` → `keys(uk) == keys(en)` (deep flatten), listing differences.
  - `App.test.tsx`: rendering at `/` shows `t('home.title')` in Ukrainian; at `/admin` shows `t('admin.title')`; at `/nope` shows `t('notFound.title')`; clicking the language switcher to `EN` changes the home title to the English text and stores `locale=en` in `localStorage`.
- [ ] **Step 3: Run** `cd web && npm test` — Expected: FAIL.
- [ ] **Step 4: Implement** the pages, router, i18n init and `LanguageSwitcher` (Mantine `SegmentedControl` with `УК` / `EN`), `MantineProvider` and `QueryClientProvider` in `main.tsx`, `api()` client.
- [ ] **Step 5: Run** `npm run lint && npm run typecheck && npm test && npm run build` — Expected: all pass; `web/dist/index.html` exists.
- [ ] **Step 6: Verify end-to-end:** `go build ./cmd/homelander` after the web build, run it, open `http://localhost:8080/admin` → the admin placeholder renders; refresh keeps it.
- [ ] **Step 7: Commit** `feat: React app shell with routing, Mantine and i18n`

### Task 6: Build, container, CI and docs

**Files:**
- Create: `Makefile`, `Dockerfile`, `.dockerignore`, `.github/workflows/ci.yml`, `.golangci.yml`, `docs/adr/README.md`, `docs/adr/0001-stack-and-process-shape.md`, `docs/development.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: all previous tasks' commands.
- Produces: `make web`, `make build` (web then `CGO_ENABLED=0 go build -trimpath -o homelander ./cmd/homelander`), `make test` (`go test -race ./...` and `npm test`), `make lint` (`golangci-lint run` and `npm run lint && npm run typecheck`), `make run` (loads `.env`).

Dockerfile: stage `web` on `node:26-alpine` (`npm ci && npm run build`); stage `go` on `golang:1.27-alpine` copying `web/dist` from stage `web`, `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`; final `gcr.io/distroless/static-debian12:nonroot`, `ENV DATABASE_PATH=/data/homelander.db`, `EXPOSE 8080`, `USER nonroot`, entrypoint the binary. CI on `pull_request` and `push` to `main`: job `go` (setup-go 1.27, `go vet ./...`, golangci-lint action v2.15, `go test -race ./...` — with an empty `web/dist/.gitkeep` the build compiles), job `web` (setup-node 26, `npm ci`, lint, typecheck, test, build), job `docker` (`docker build .`, needs both). Workflow `permissions: contents: read`.

ADR-0001 records the decisions already made in the spec section 4 (one binary, Go + SQLite pure Go, React + TS, web-first identity with messenger as a channel) with links to the spec. `docs/development.md`: prerequisites, `.env` setup, `make` targets, dev mode (`go run` + `npm run dev`). README: what the project is (one paragraph, no community specifics), quick start, link to docs.

- [ ] **Step 1: Write** Makefile, `.golangci.yml` (default linters plus `errorlint`, `gosec`, `revive`), Dockerfile, `.dockerignore` (`.git`, `web/node_modules`, `data`, `.env`).
- [ ] **Step 2: Verify** `make lint && make test && make build` — Expected: all pass.
- [ ] **Step 3: Verify the image:** `docker build -t homelander:dev . && docker run --rm -e APP_SECRET=$(openssl rand -hex 32) -e PUBLIC_URL=http://localhost:8080 -e TELEGRAM_BOT_TOKEN=000000:placeholder -p 8080:8080 -v homelander-data:/data homelander:dev` then `curl -s localhost:8080/healthz` → `{"status":"ok"}` and `curl -s localhost:8080/ | grep -c '<div id="root">'` → `1`.
- [ ] **Step 4: Write** the CI workflow, ADR-0001, `docs/adr/README.md` (index + template pointer), `docs/development.md`, README.
- [ ] **Step 5: Commit** `chore: Makefile, Dockerfile, CI and developer docs`
- [ ] **Step 6: Push the branch, open the PR, confirm all three CI jobs are green.**
