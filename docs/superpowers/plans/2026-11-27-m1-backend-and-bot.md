# M1 Backend and Bot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Registration applications, admin review, activation through the Telegram bot, login with a Telegram code, server-side sessions, and the full JSON API the M1 web screens (plan 3) consume.

**Architecture:** `internal/domain` holds entities, errors, repository interfaces (declared by the consumer) and services; `internal/storage/sqlite` implements the repositories; `internal/messenger/telegram` turns Telegram updates into domain calls and implements `domain.Messenger`; `internal/httpapi` exposes the services as JSON. Everything is wired in `cmd/homelander`. Domain tests run against a real temporary SQLite database.

**Tech Stack:** Go 1.27, `modernc.org/sqlite`, `github.com/PaulSonOfLars/gotgbot/v2` v2.0.0-rc.37, `github.com/nyaruka/phonenumbers` v1.8.1, `nicksnyder/go-i18n/v2`, `net/http`.

**Spec:** `docs/superpowers/specs/2026-11-27-m0-m1-foundation-and-identity-design.md` (sections 2, 3, 5.2–5.5, 5.7, 5.8, 5.10). Builds on M0 (`docs/superpowers/plans/2026-11-27-m0-foundation.md`).

## Global Constraints

- Every user-facing text (bot replies, error texts the API renders) comes from `locales/uk.json` and `locales/en.json`, both with the same keys; no Ukrainian or English literals in Go code outside tests. API errors are **codes**, never sentences; the web translates them.
- Admin-entered content is stored per locale as JSON `{"uk": "...", "en": "..."}`.
- No real phone numbers, addresses or names in code, tests or fixtures. Test phones: `+380500000001`…`+380500000099`.
- Phones are stored and compared only in E.164 form, normalized with `DEFAULT_PHONE_REGION`.
- Repositories run every statement through `sqlite.DB.Q(ctx)` and close every `*sql.Rows` (one connection; anything else can deadlock inside `InTx`).
- Secrets (codes, session tokens) are stored only as HMAC-SHA256 with `APP_SECRET`, compared in constant time, never logged.
- **Every schema change is a new migration file** `internal/storage/sqlite/migrations/NNNN_name.sql` (M1 adds `0002_identity.sql`; later changes get `0003_…`); an applied migration is never edited. `TestEmbeddedMigrationsApply` and each repository test run on a freshly migrated database.
- Times are UTC, stored as RFC 3339 text; services take a `domain.Clock` so tests control time.
- All API errors use `{"error":{"code":"...","fields":{...}}}`; `fields` only for `validation`.
- Work on a branch, PR into `main`.

## Review Focus

1. A forwarded contact (`contact.user_id ≠ from.id`) in the bot never links anything, even when the phone matches an `awaiting_activation` user (Task 9 `TestContactFromSomeoneElseIsRefused`).
2. Two concurrent `POST /auth/verify` with the right code → exactly one session (Task 8 `TestVerifyCodeIsSingleUse`).
3. Every registered-or-not phone gets byte-identical responses from `POST /register` and `POST /auth/code` (Task 11 `TestRegisterSameAnswerForKnownPhone`, `TestAuthCodeSameAnswer`).
4. Removing the last active admin by any path — revoke flag, disable, unlink channel — is refused (Task 6 `TestLastAdminProtected`).
5. A Telegram contact phone written without `+` (Telegram sends `380…`) still matches the stored E.164 number (Task 2 `TestNormalizePhone`).

---

### Task 1: Domain foundations — clock, errors, audit, rate limiter

**Files:**
- Create: `internal/domain/clock.go`, `internal/domain/errors.go`, `internal/domain/audit.go`, `internal/ratelimit/ratelimit.go`, `internal/ratelimit/ratelimit_test.go`

**Interfaces:**
- Produces:
  ```go
  package domain
  type Clock func() time.Time
  // Errors mapped to HTTP codes by httpapi (Task 11).
  var (
      ErrNotFound      = errors.New("not_found")
      ErrConflict      = errors.New("conflict")
      ErrForbidden     = errors.New("forbidden")
      ErrRateLimited   = errors.New("rate_limited")
      ErrTryLater      = errors.New("try_later")
      ErrInvalidCode   = errors.New("invalid_code")
      ErrUnauthorized  = errors.New("unauthorized")
      ErrLastAdmin     = errors.New("last_admin")
  )
  // ValidationError carries field → reason codes ("required", "invalid", "out_of_range", "taken").
  type ValidationError struct{ Fields map[string]string }
  func (e *ValidationError) Error() string
  type AuditEntry struct { ID int64; At time.Time; ActorUserID *int64; Action, Target string; Details map[string]any; IP string }
  type AuditRepo interface {
      Add(ctx context.Context, e AuditEntry) error
      List(ctx context.Context, f AuditFilter) ([]AuditEntry, int, error)
  }
  type AuditFilter struct { TargetUserID *int64; Limit, Offset int }

  package ratelimit
  type Limiter struct{ /* mutex, map[key][]time.Time, clock */ }
  func New(now func() time.Time) *Limiter
  // Allow records a hit for key and reports whether it is within limit hits per window.
  func (l *Limiter) Allow(key string, limit int, window time.Duration) bool
  ```
  Audit `Target` format: `"user:<id>"`, `"house:<id>"`, `"status:<id>"`, `"site"`. Action names: `user.applied`, `user.approved`, `user.rejected`, `user.created`, `user.updated`, `user.disabled`, `user.enabled`, `user.admin_granted`, `user.admin_revoked`, `user.resident_set`, `user.resident_unset`, `channel.linked`, `channel.unlinked`, `auth.code_sent`, `auth.login`, `auth.login_failed`, `auth.logout`, `house.created`, `house.updated`, `status.created`, `status.updated`, `site.updated`, `site.picture_set`, `site.picture_removed`, `bootstrap.admin_created`.

- [ ] **Step 1: Write failing tests** `TestAllowWithinLimit` (3 hits of limit 3 allowed, 4th refused), `TestAllowWindowSlides` (after window passes on the fake clock, allowed again), `TestAllowKeysIndependent`, `TestAllowForgetsOldKeys` (after 2×window an entry for an idle key is gone — check via an exported-for-test `l.size()` in `export_test.go`).
- [ ] **Step 2: Run** `go test ./internal/ratelimit/` — Expected: FAIL.
- [ ] **Step 3: Implement** the limiter (sliding log per key, prune on each call; a key with no hits in its window is deleted) and the domain files.
- [ ] **Step 4: Run** `go test -race ./internal/ratelimit/ ./internal/domain/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: domain errors, audit types and an in-memory rate limiter`

### Task 2: Phone normalization

**Files:**
- Create: `internal/domain/phone.go`, `internal/domain/phone_test.go`

**Interfaces:**
- Produces: `func NormalizePhone(raw, region string) (string, error)` — returns E.164 (`+380501234567`) or `ErrInvalidPhone` (`var ErrInvalidPhone = errors.New("invalid_phone")`). Uses `phonenumbers.Parse` + `IsValidNumber` + `Format(…, phonenumbers.E164)`. Input `380…` without `+` (how Telegram sends contacts) is tried as `+380…` first when its digits start with a country code and the national parse is invalid.

- [ ] **Step 1: Write failing test** `TestNormalizePhone` table (region `UA`): `"050 000 00 01"` → `"+380500000001"`; `"+380500000001"` → same; `"380500000001"` → same; `"0500000001"` → same; `"(050) 000-00-01"` → same; `"+48 600 000 001"` → `"+48600000001"`; `""`, `"12"`, `"abc"` → `ErrInvalidPhone`.
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement**; `go get github.com/nyaruka/phonenumbers@v1.8.1`.
- [ ] **Step 4: Run** `go test ./internal/domain/ -run TestNormalizePhone` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: phone normalization to E.164`

### Task 3: Identity schema and user/channel repositories

**Files:**
- Create: `internal/storage/sqlite/migrations/0002_identity.sql`, `internal/domain/user.go`, `internal/storage/sqlite/users.go`, `internal/storage/sqlite/users_test.go`, `internal/storage/sqlite/sqlitetest/sqlitetest.go`

**Interfaces:**
- Consumes: `sqlite.DB`, `DB.Q`, `DB.InTx` (M0).
- Produces:
  ```go
  package domain
  type UserStatus string
  const (StatusPendingApproval UserStatus = "pending_approval"; StatusAwaitingActivation = "awaiting_activation"; StatusActive = "active"; StatusRejected = "rejected"; StatusDisabled = "disabled")
  type UserSource string
  const (SourceApplication UserSource = "application"; SourceAdmin = "admin"; SourceBootstrap = "bootstrap")
  type User struct {
      ID int64; Phone, FirstName, LastName, Locale string
      IsResident, IsAdmin bool; Status UserStatus; Source UserSource
      ApprovedBy *int64; ApprovedAt, ActivatedAt *time.Time; CreatedAt, UpdatedAt time.Time
  }
  type Channel struct { ID, UserID int64; Type, ExternalID, ChatID, Username string; LinkedAt time.Time }
  type UserFilter struct { Query string; HouseID *int64; Status *UserStatus; IsAdmin, IsResident *bool; Limit, Offset int }
  type UserRepo interface {
      Create(ctx context.Context, u *User) error                       // sets ID, CreatedAt, UpdatedAt
      Update(ctx context.Context, u *User) error                       // all mutable fields, bumps UpdatedAt
      Get(ctx context.Context, id int64) (User, error)                 // ErrNotFound
      GetByPhone(ctx context.Context, phone string) (User, error)      // ErrNotFound
      List(ctx context.Context, f UserFilter) ([]User, int, error)     // ordered by last_name, first_name, id
      CountActiveAdmins(ctx context.Context) (int, error)
      CountByStatus(ctx context.Context, s UserStatus) (int, error)
  }
  type ChannelRepo interface {
      Create(ctx context.Context, c *Channel) error                    // ErrConflict on (type, external_id) or (user_id, type)
      ListByUser(ctx context.Context, userID int64) ([]Channel, error)
      GetByExternal(ctx context.Context, typ, externalID string) (Channel, error)
      Delete(ctx context.Context, id int64) error
  }

  package sqlite
  type Users struct{ DB DB };    func (Users) …    // implements domain.UserRepo
  type Channels struct{ DB DB }; func (Channels) … // implements domain.ChannelRepo

  package sqlitetest
  // New opens a migrated database in t.TempDir(); used by sqlite, domain and httpapi tests.
  func New(t testing.TB) sqlite.DB
  ```

`0002_identity.sql` creates every M1 table of spec 5.2 as STRICT tables: `users` (phone UNIQUE, CHECK on status/source values, is_resident/is_admin INTEGER 0/1, approved_by REFERENCES users), `channels` (UNIQUE(type, external_id), UNIQUE(user_id, type), ON DELETE CASCADE), `houses` (name, address, latitude REAL NULL, longitude REAL NULL, floors INTEGER NULL, apartment_from, apartment_to with CHECK from ≥ 1 AND to ≥ from, sort_order, archived), `residency_statuses` (code UNIQUE, names TEXT CHECK json_valid, sort_order, archived) seeded with `owner` `{"uk":"Власник","en":"Owner"}` and `tenant` `{"uk":"Орендар","en":"Tenant"}`, `residencies` (UNIQUE(user_id, house_id, apartment_number), FKs, ON DELETE CASCADE from users), `login_codes`, `sessions` (token_hash UNIQUE), `audit_log` (index on target, at), `assets` (data BLOB). Index `users(status)`, `residencies(house_id, apartment_number)`.

`List` with `Query` matches case-insensitively on first name, last name, `first || ' ' || last`, `last || ' ' || first`, and on phone digits (the query's digits contained in the phone); `HouseID` joins residencies.

- [ ] **Step 1: Write failing tests** in `users_test.go`: `TestUserCreateGet`, `TestUserPhoneUnique` (second Create with the same phone → `domain.ErrConflict`), `TestUserGetMissing` (`ErrNotFound`), `TestUserListSearch` (users Іван Петренко / Olena Shevchenko / Петро Іваненко: query `"петренко"` → 1, `"іван"` → Іван Петренко and Петро Іваненко, `"0500000002"` → Olena, `"Петренко Іван"` → 1; `total` matches), `TestUserListFilters` (status, isAdmin, houseId), `TestCountActiveAdmins` (disabled admin not counted), `TestChannelUniqueness` (same external id twice → ErrConflict; second telegram channel for one user → ErrConflict), `TestSeededStatuses` (two rows, codes owner/tenant).
- [ ] **Step 2: Run** `go test ./internal/storage/sqlite/` — Expected: FAIL.
- [ ] **Step 3: Implement** the migration, entities and repositories. Map SQLite unique violations (`sqlite3.SQLITE_CONSTRAINT_UNIQUE`, via `errors.As` on `*sqlite.Error` code 2067) to `domain.ErrConflict` in one helper `mapErr(err error) error` reused by later repositories.
- [ ] **Step 4: Run** `go test -race ./internal/storage/sqlite/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: identity schema with user and channel repositories`

### Task 4: Houses, statuses, residencies repositories and house statistics

**Files:**
- Create: `internal/domain/house.go`, `internal/storage/sqlite/houses.go`, `internal/storage/sqlite/houses_test.go`

**Interfaces:**
- Produces:
  ```go
  package domain
  type LocalizedText map[string]string // locale → text
  type House struct {
      ID int64; Name, Address string; Latitude, Longitude *float64; Floors *int
      ApartmentFrom, ApartmentTo, SortOrder int; Archived bool
  }
  type ResidencyStatus struct { ID int64; Code string; Names LocalizedText; SortOrder int; Archived bool }
  type Residency struct { ID, UserID, HouseID int64; Apartment int; StatusID int64; CreatedAt time.Time }
  type HouseStats struct {
      ApartmentsTotal, ApartmentsWithResidents, People int
      ByResidencyStatus map[int64]int      // status id → people
      ByUserStatus map[UserStatus]int
      Apartments map[int]int               // apartment number → people; only non-zero
  }
  type ApartmentResident struct { User User; Residency Residency }
  type HouseRepo interface {
      Create(ctx context.Context, h *House) error
      Update(ctx context.Context, h *House) error
      Get(ctx context.Context, id int64) (House, error)
      List(ctx context.Context, includeArchived bool) ([]House, error)  // sort_order, name
      Stats(ctx context.Context, houseID int64) (HouseStats, error)
      Residents(ctx context.Context, houseID int64, apartment int) ([]ApartmentResident, error)
      ResidenciesOutside(ctx context.Context, houseID int64, from, to int) (int, error)
  }
  type StatusRepo interface {
      Create(ctx context.Context, s *ResidencyStatus) error   // ErrConflict on code
      Update(ctx context.Context, s *ResidencyStatus) error
      Get(ctx context.Context, id int64) (ResidencyStatus, error)
      List(ctx context.Context, includeArchived bool) ([]ResidencyStatus, error)
  }
  type ResidencyRepo interface {
      Create(ctx context.Context, r *Residency) error         // ErrConflict on duplicate
      Update(ctx context.Context, r *Residency) error
      Get(ctx context.Context, id int64) (Residency, error)
      Delete(ctx context.Context, id int64) error
      ListByUser(ctx context.Context, userID int64) ([]Residency, error)
  }

  package sqlite
  type Houses struct{ DB DB }; type Statuses struct{ DB DB }; type Residencies struct{ DB DB }
  ```

`Stats` counts only users with `is_resident = 1` and status in `pending_approval`, `awaiting_activation`, `active`; `People` counts distinct users; `ApartmentsTotal = to − from + 1`.

- [ ] **Step 1: Write failing tests:** `TestHouseCRUD`, `TestHouseListHidesArchived`, `TestStatusNamesRoundTrip` (names `{"uk":"Співвласник","en":"Co-owner"}` survive), `TestStatusCodeUnique`, `TestResidencyDuplicate` (same user/house/apartment → ErrConflict; a second user in the same apartment → OK), `TestHouseStats` (house 1–10; apartment 3: two active residents (owner, tenant); apartment 5: one pending; apartment 7: one rejected and one disabled; one admin with `is_resident = 0` in apartment 8 → `ApartmentsTotal 10, ApartmentsWithResidents 2, People 3, Apartments {3:2, 5:1}, ByUserStatus {active:2, pending_approval:1}, ByResidencyStatus {owner:2, tenant:1}`), `TestResidentsOfApartment`, `TestResidenciesOutside` (range shrunk to 1–4 → 1 residency outside).
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/storage/sqlite/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: houses, residency statuses, residencies and house statistics`

### Task 5: Codes, sessions, audit, settings and assets repositories

**Files:**
- Create: `internal/domain/auth.go`, `internal/domain/site.go`, `internal/storage/sqlite/auth.go`, `internal/storage/sqlite/site.go`, `internal/storage/sqlite/audit.go`, tests `auth_test.go`, `site_test.go`, `audit_test.go`

**Interfaces:**
- Produces:
  ```go
  package domain
  type LoginCode struct { ID, UserID int64; CodeHash []byte; ExpiresAt time.Time; Attempts int; UsedAt *time.Time; IP string }
  type Session struct { ID, UserID int64; TokenHash []byte; CreatedAt, LastSeenAt, ExpiresAt time.Time; IP, UserAgent string }
  type CodeRepo interface {
      Replace(ctx context.Context, c *LoginCode) error              // deletes the user's other unused codes, inserts c
      Active(ctx context.Context, userID int64, now time.Time) (LoginCode, error) // unused, not expired; ErrNotFound
      IncAttempts(ctx context.Context, id int64) (int, error)       // returns new count
      MarkUsed(ctx context.Context, id int64, at time.Time) (bool, error) // false if it was already used (single use under races)
  }
  type SessionRepo interface {
      Create(ctx context.Context, s *Session) error
      GetByHash(ctx context.Context, hash []byte) (Session, error)
      Touch(ctx context.Context, id int64, at time.Time) error
      Delete(ctx context.Context, id int64) error
      DeleteByUser(ctx context.Context, userID int64) error
      DeleteExpired(ctx context.Context, now time.Time) (int, error)
  }
  type SiteSettings struct { CommunityName, WelcomeText LocalizedText; DefaultLocale string; PictureAssetID *int64 }
  type Asset struct { ID int64; Kind, ContentType string; Data []byte; SHA256 string; UploadedBy int64; UploadedAt time.Time }
  type SiteRepo interface {
      Get(ctx context.Context) (SiteSettings, error)               // zero value with empty maps when unset
      Save(ctx context.Context, s SiteSettings) error              // one JSON row under key "site"
      PutAsset(ctx context.Context, a *Asset) error
      GetAsset(ctx context.Context, id int64) (Asset, error)
      DeleteAsset(ctx context.Context, id int64) error
  }
  package sqlite
  type Codes struct{ DB DB }; type Sessions struct{ DB DB }; type Site struct{ DB DB }; type Audit struct{ DB DB }
  ```

- [ ] **Step 1: Write failing tests:** `TestCodeReplaceKeepsOne`, `TestCodeActiveIgnoresExpiredAndUsed`, `TestMarkUsedOnlyOnce` (second call returns false), `TestSessionLifecycle` (create, get by hash, touch, delete), `TestDeleteSessionsByUser`, `TestDeleteExpiredSessions`, `TestSiteDefaultsWhenUnset`, `TestSiteRoundTrip`, `TestAssetRoundTrip`, `TestAuditListByTargetUser` (newest first, total count).
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement.** `MarkUsed` is `UPDATE … SET used_at = ? WHERE id = ? AND used_at IS NULL` and returns `RowsAffected() == 1`.
- [ ] **Step 4: Run** `go test -race ./internal/storage/sqlite/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: repositories for codes, sessions, site settings, assets and audit`

### Task 6: User service — applications, review, admin-managed users

**Files:**
- Create: `internal/domain/users_service.go`, `internal/domain/users_service_test.go`, `internal/domain/testenv_test.go`

**Interfaces:**
- Consumes: repos from Tasks 3–5, `NormalizePhone`, `ratelimit.Limiter`, `storage.TxRunner`.
- Produces:
  ```go
  package domain
  type Repos struct {
      Tx storage.TxRunner; Users UserRepo; Channels ChannelRepo; Houses HouseRepo; Statuses StatusRepo
      Residencies ResidencyRepo; Codes CodeRepo; Sessions SessionRepo; Site SiteRepo; Audit AuditRepo
  }
  type Actor struct { UserID *int64; IP string }   // nil UserID = the public or the bot
  type Application struct { Phone, FirstName, LastName string; HouseID int64; Apartment int; StatusID int64; Consent bool; Locale string }
  type NewUser struct { Phone, FirstName, LastName string; IsResident, IsAdmin bool }
  type UserPatch struct { FirstName, LastName, Phone, Locale *string; IsResident, IsAdmin *bool }
  type UserCard struct { User User; Channels []Channel; Residencies []Residency; Audit []AuditEntry }
  type UserService struct { R Repos; Now Clock; Limits *ratelimit.Limiter; Region string; PendingCap int }
  func (s *UserService) Apply(ctx context.Context, a Application, actor Actor) error
  func (s *UserService) Approve(ctx context.Context, id int64, actor Actor) error
  func (s *UserService) Reject(ctx context.Context, id int64, actor Actor) error
  func (s *UserService) AddUser(ctx context.Context, n NewUser, actor Actor) (User, error)
  func (s *UserService) Update(ctx context.Context, id int64, p UserPatch, actor Actor) (User, error)
  func (s *UserService) Disable(ctx context.Context, id int64, actor Actor) error
  func (s *UserService) Enable(ctx context.Context, id int64, actor Actor) error
  func (s *UserService) UnlinkChannel(ctx context.Context, userID, channelID int64, actor Actor) error
  func (s *UserService) AddResidency(ctx context.Context, userID int64, r Residency, actor Actor) (Residency, error)
  func (s *UserService) UpdateResidency(ctx context.Context, r Residency, actor Actor) error
  func (s *UserService) RemoveResidency(ctx context.Context, id int64, actor Actor) error
  func (s *UserService) Card(ctx context.Context, id int64) (UserCard, error)   // last 20 audit entries for the user
  func (s *UserService) List(ctx context.Context, f UserFilter) ([]User, int, error)
  // SessionsEnded is called after changes that must end a user's sessions (wired to SessionRepo.DeleteByUser).
  ```

Rules (spec 5.2 diagram, 5.4):
- `Apply`: rate limit key `apply:<ip>` 5/hour → `ErrRateLimited`; validation (fields `phone` invalid, `firstName`/`lastName` required, max 100 chars, `houseId` unknown or archived → `invalid`, `apartment` outside range → `out_of_range`, `statusId` unknown/archived → `invalid`, `consent` must be true → `required`) → `*ValidationError`; pending count ≥ `PendingCap` (200) → `ErrTryLater`; phone already used by a non-`rejected` user → return **nil** and change nothing; `rejected` user → reuse the row: set names, `pending_approval`, `is_resident = true`, replace residencies with the new one; else create `pending_approval`, `source = application`, `is_resident = true` + residency. Audit `user.applied`.
- `Approve`/`Reject` only from `pending_approval` (else `ErrConflict`); approve → `awaiting_activation`, `approved_by`, `approved_at`.
- `AddUser`: phone normalized; taken → `ValidationError{phone: taken}`; status `awaiting_activation`, source `admin`.
- `Update`: phone change only when the user has no channel (else `ValidationError{phone: "linked"}`); phone change, admin revoke → end sessions; revoking admin or `Disable` of the last active admin → `ErrLastAdmin`; `UnlinkChannel` of the last active admin's only channel → `ErrLastAdmin`; unlink sets `awaiting_activation` (when the user was `active`) and ends sessions. `Disable` ends sessions; `Enable` returns to `active` if a channel exists, else `awaiting_activation`.
- Residency changes validate house, range and status as `Apply` does.
- Every change writes one audit entry with the actor; flag changes use the specific actions.

- [ ] **Step 1: Write failing tests** (`testenv_test.go`, package `domain_test`, builds `Repos` over `sqlitetest.New(t)` from Task 3): `TestApplyCreatesPendingResident`, `TestApplyValidation` (each field reason), `TestApplyKnownPhoneChangesNothing` (existing active user: returns nil, names unchanged, no new residency, no audit entry), `TestApplyRejectedReapplies`, `TestApplyRateLimited` (6th from one IP), `TestApplyPendingCap` (cap 2), `TestApplySecondResidentSameApartment`, `TestApproveReject` (and from wrong status → ErrConflict), `TestAddUserAwaitsActivation`, `TestPhoneEditRules` (unlinked OK and normalized; linked → validation `linked`), `TestLastAdminProtected` (revoke flag, disable, unlink — each → ErrLastAdmin while only one active admin; allowed once a second active admin exists), `TestSessionsEndedOnSensitiveChanges` (admin revoke, disable, unlink, phone change each delete the user's sessions), `TestEnableRestoresStatus`, `TestResidencyOutOfRange`, `TestCardHasAuditNewestFirst`.
- [ ] **Step 2: Run** `go test ./internal/domain/` — Expected: FAIL.
- [ ] **Step 3: Implement** the service; each public method runs inside `R.Tx.InTx`.
- [ ] **Step 4: Run** `go test -race ./internal/domain/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: user service with applications, review and admin-managed users`

### Task 7: Houses, statuses and site services

**Files:**
- Create: `internal/domain/catalog_service.go`, `internal/domain/catalog_service_test.go`

**Interfaces:**
- Produces:
  ```go
  type CatalogService struct { R Repos; Now Clock; Locales []string }
  func (s *CatalogService) CreateHouse(ctx context.Context, h House, actor Actor) (House, error)
  func (s *CatalogService) UpdateHouse(ctx context.Context, h House, actor Actor) (House, error)
  func (s *CatalogService) Houses(ctx context.Context, includeArchived bool) ([]House, error)
  func (s *CatalogService) HouseDetail(ctx context.Context, id int64) (House, HouseStats, error)
  func (s *CatalogService) ApartmentResidents(ctx context.Context, houseID int64, apartment int) ([]ApartmentResident, error)
  func (s *CatalogService) CreateStatus(ctx context.Context, st ResidencyStatus, actor Actor) (ResidencyStatus, error)
  func (s *CatalogService) UpdateStatus(ctx context.Context, st ResidencyStatus, actor Actor) (ResidencyStatus, error)
  func (s *CatalogService) Statuses(ctx context.Context, includeArchived bool) ([]ResidencyStatus, error)
  func (s *CatalogService) Site(ctx context.Context) (SiteSettings, error)
  func (s *CatalogService) SaveSite(ctx context.Context, st SiteSettings, actor Actor) (SiteSettings, error)
  func (s *CatalogService) SetPicture(ctx context.Context, contentType string, data []byte, actor Actor) (Asset, error)
  func (s *CatalogService) RemovePicture(ctx context.Context, actor Actor) error
  ```

Rules: house `name` required (≤ 100), `apartmentFrom ≥ 1`, `apartmentTo ≥ apartmentFrom`, `apartmentTo − apartmentFrom < 2000`, `floors` 1–200 when set, latitude −90..90, longitude −180..180; narrowing a range so residencies fall outside → `ValidationError{apartmentTo|apartmentFrom: "has_residents"}`. Status `code` `^[a-z][a-z0-9_]{1,31}$`, names required for the default locale. Site: `DefaultLocale` must be supported; names/welcome keys limited to supported locales, each ≤ 200 / 2000 chars. Picture: content type detected with `http.DetectContentType` and must be `image/png`, `image/jpeg` or `image/webp` (an SVG or anything else → `ValidationError{picture: "type"}`), size ≤ 5 MiB (`picture: "size"`); replacing deletes the previous asset.

- [ ] **Step 1: Write failing tests:** `TestHouseValidation`, `TestNarrowRangeWithResidentsRefused`, `TestStatusValidation`, `TestSiteValidation`, `TestPictureTypes` (PNG header bytes OK; `<svg …>` refused; 5 MiB + 1 refused), `TestPictureReplaceDeletesOld`, `TestCatalogAudit`.
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/domain/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: houses, statuses and site settings services`

### Task 8: Auth service — codes, sessions, activation, bootstrap

**Files:**
- Create: `internal/domain/messenger.go`, `internal/domain/auth_service.go`, `internal/domain/auth_service_test.go`, `internal/domain/fakemessenger_test.go`

**Interfaces:**
- Produces:
  ```go
  // Messenger is implemented by internal/messenger/telegram; texts arrive already localized.
  type Messenger interface {
      Type() string
      SendText(ctx context.Context, chatID, text string) error
      BotURL() string
  }
  type Translator interface { T(locale, id string, data map[string]any, count any) string; Pick(c ...string) string }
  type ActivationResult string
  const (
      Activated ActivationResult = "activated"; AlreadyConnected = "already_connected"
      PendingApproval = "pending_approval"; NotRegistered = "not_registered"; BootstrapAdmin = "bootstrap_admin"
  )
  type AuthService struct {
      R Repos; Now Clock; Limits *ratelimit.Limiter; Secret []byte; Region string
      Messenger Messenger; I18n Translator; BootstrapPhone string
      CodeTTL time.Duration      // 5m
      MaxAttempts int            // 5
      IdleTTL, MaxTTL time.Duration
      Rand io.Reader             // crypto/rand.Reader; tests pass a fixed reader
  }
  func (s *AuthService) ActivateChannel(ctx context.Context, typ, externalID, chatID, username, verifiedPhone string) (ActivationResult, User, error)
  func (s *AuthService) RequestCode(ctx context.Context, phone string, actor Actor) error
  func (s *AuthService) VerifyCode(ctx context.Context, phone, code string, ua string, actor Actor) (User, string, error) // raw session token
  func (s *AuthService) Authenticate(ctx context.Context, rawToken string) (User, Session, error) // ErrUnauthorized
  func (s *AuthService) Logout(ctx context.Context, rawToken string, actor Actor) error
  func HashSecret(secret []byte, value string) []byte // HMAC-SHA256
  ```

Rules (spec 5.4): `ActivateChannel` — rate limit `activate:<externalID>` 5/hour → `ErrRateLimited`; phone normalized; bootstrap first: if `BootstrapPhone` set, normalized equal, and `CountActiveAdmins == 0` and no user with that phone is active → create (or reuse) user `active`, admin, not resident, source `bootstrap`, link channel, audit `bootstrap.admin_created` → `BootstrapAdmin`; else by status: `awaiting_activation` and no channel of this type → link, `active`, `activated_at` → `Activated`; `active` and this external id is its channel → `AlreadyConnected`; `pending_approval` → `PendingApproval`; everything else → `NotRegistered`. The `contact.user_id == from.id` check happens in the Telegram adapter (Task 9) before this call.
`RequestCode` — rate limits `code:<phone>` 3/15 min and `codeip:<ip>` 10/15 min → `ErrRateLimited`; invalid phone → `ValidationError{phone: invalid}`; sends only for an `active` user with a channel: 6 digits from `Rand` (uniform, `000000`–`999999`), `CodeRepo.Replace`, message key `bot.login.code` with `{{.Code}}` and `{{.Time}}` (`15:04 UTC`) in the user's locale, audit `auth.code_sent`; otherwise returns nil having done nothing.
`VerifyCode` — rate limit `verifyip:<ip>` 20/15 min; finds the active code; wrong code → `IncAttempts`, at `MaxAttempts` the code is marked used, audit `auth.login_failed`, `ErrInvalidCode`; right code → `MarkUsed` must return true (else `ErrInvalidCode`), session token 32 bytes from `Rand`, base64url; session `ExpiresAt = now + MaxTTL`; audit `auth.login`.
`Authenticate` — unknown hash, `now > ExpiresAt`, `now − LastSeenAt > IdleTTL`, or user not `active` → delete the session if it exists, `ErrUnauthorized`; else `Touch` when `LastSeenAt` is older than one minute.

- [ ] **Step 1: Write failing tests:** `TestActivateAwaitingUser`, `TestActivateStatuses` (pending → PendingApproval; rejected, disabled, unknown → NotRegistered; active with another Telegram id → NotRegistered; same id → AlreadyConnected), `TestActivatePhoneWithoutPlus`, `TestActivateRateLimited`, `TestBootstrapOnlyWithoutAdmins` (second activation from the bootstrap phone after an admin exists → normal rules), `TestRequestCodeSendsOnlyToActive` (fake messenger records 1 message for active, 0 for pending/disabled/unknown/no channel; all return nil), `TestCodeMessageLocalized` (user locale `en` → English text containing the code), `TestVerifyCodeWrongThenRight`, `TestVerifyCodeExpires` (clock +5m1s → ErrInvalidCode), `TestVerifyCodeAttemptsExhausted` (5 wrong → the right code is refused), `TestVerifyCodeIsSingleUse` (two goroutines verify the same right code → exactly one session row), `TestNewCodeInvalidatesOld`, `TestAuthenticateIdleAndMax` (idle 12h, max 168h on the fake clock), `TestAuthenticateDisabledUser`, `TestLogoutDeletesSession`.
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement.** Add locale keys `bot.login.code` (uk/en) to `locales/*.json`.
- [ ] **Step 4: Run** `go test -race ./internal/domain/ ./internal/i18n/` — Expected: PASS (parity test included).
- [ ] **Step 5: Commit** `feat: login codes, sessions, channel activation and bootstrap admin`

### Task 9: Telegram adapter

**Files:**
- Create: `internal/messenger/telegram/telegram.go`, `internal/messenger/telegram/updates.go`, `internal/messenger/telegram/webhook.go`, `internal/messenger/telegram/telegram_test.go`
- Modify: `locales/uk.json`, `locales/en.json`, `internal/httpapi/server.go` (add `Mount`)

**Interfaces:**
- Consumes: `domain.AuthService.ActivateChannel`, `i18n.Bundle`, `config.Config`.
- Produces:
  ```go
  package telegram
  // API is the subset of *gotgbot.Bot used here; tests use a fake.
  type API interface {
      GetMe(opts *gotgbot.GetMeOpts) (*gotgbot.User, error)
      GetUpdates(opts *gotgbot.GetUpdatesOpts) ([]gotgbot.Update, error)
      SendMessage(chatID int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error)
      SetWebhook(url string, opts *gotgbot.SetWebhookOpts) (bool, error)
      DeleteWebhook(opts *gotgbot.DeleteWebhookOpts) (bool, error)
  }
  type Activator interface {
      ActivateChannel(ctx context.Context, typ, externalID, chatID, username, verifiedPhone string) (domain.ActivationResult, domain.User, error)
  }
  // GroupMessage is a messenger-neutral copy of a message seen in a group or supergroup.
  // M1 only hands it to the sink; reactions are configured later as flows (M5) running WASM modules (M3).
  type GroupMessage struct {
      ChatID, ChatTitle, MessageID, ThreadID string // ThreadID: forum topic, "" outside topics
      FromExternalID, FromUsername, FromName, LanguageCode string
      Text string; At time.Time
  }
  // GroupSink receives group messages. M1 wires NopSink: nothing is answered, stored or logged beyond debug counters.
  type GroupSink interface { OnGroupMessage(ctx context.Context, m GroupMessage) }
  type NopSink struct{}
  type Bot struct { /* api, activator, sink, i18n, publicURL, username, log */ }
  func New(api API, act Activator, sink GroupSink, tr *i18n.Bundle, publicURL string, log *slog.Logger) (*Bot, error) // calls GetMe for the username
  func (b *Bot) Type() string                    // "telegram"
  func (b *Bot) BotURL() string                  // "https://t.me/<username>"
  func (b *Bot) SendText(ctx context.Context, chatID, text string) error
  func (b *Bot) HandleUpdate(ctx context.Context, u gotgbot.Update)
  func (b *Bot) RunPolling(ctx context.Context) error       // DeleteWebhook first; GetUpdates timeout 50 s; offset = last update_id + 1
  func (b *Bot) StartWebhook(ctx context.Context, url, secret string) error // SetWebhook with SecretToken, AllowedUpdates ["message"]
  func WebhookPath(secret []byte) string                    // "/telegram/webhook/" + first 32 hex chars of HMAC(secret, "telegram-webhook")
  func (b *Bot) WebhookHandler(secret string) http.Handler  // 401 unless X-Telegram-Bot-Api-Secret-Token matches (subtle.ConstantTimeCompare); body ≤ 1 MiB; always 200 after decoding

  package httpapi
  func (s *Server) Mount(prefix string, h http.Handler) // routes paths with prefix before the SPA
  ```

Behaviour: `message` updates in **private** chats are handled as below. `message` updates in `group` / `supergroup` chats are converted to `GroupMessage` and passed to the sink, and the bot **never replies in a group** in M1 (how it reacts is decided later, in the visual flow editor). Other update kinds are ignored. Note for deployment docs: to see every group message the bot needs privacy mode turned off in @BotFather (or admin rights in the group); with privacy mode on, Telegram delivers only commands and replies to the bot. `/start` (with or without payload) → `bot.start.prompt` with a one-time reply keyboard of one button `bot.start.button` (`RequestContact: true`, `ResizeKeyboard`, `OneTimeKeyboard`). A contact with `UserId != From.Id` or no `UserId` → `bot.contact.notOwn`, no domain call. Own contact → `ActivateChannel("telegram", fromID, chatID, username, contact.PhoneNumber)`; reply by result with `ReplyKeyboardRemove`: `Activated` → `bot.activated` (`{{.Name}}`, `{{.URL}}` = `PUBLIC_URL`), `BootstrapAdmin` → `bot.bootstrap` (`{{.URL}}` = `PUBLIC_URL + "/admin"`), `AlreadyConnected` → `bot.alreadyConnected`, `PendingApproval` → `bot.pending`, `NotRegistered` → `bot.notRegistered`; `ErrRateLimited` → `bot.rateLimited`; other errors → `common.error.internal` and an error log. Any other text → `bot.help`. Language: the linked user's `Locale` when the result has a user, else `Pick(from.LanguageCode)`. `SendText` parses `chatID` as int64. Polling backs off 1 s → 30 s on errors and stops on `ctx.Done()`.

- [ ] **Step 1: Write failing tests** with a fake `API` (records sent messages, serves queued updates) and a fake `Activator`: `TestStartAsksForContact` (one message, keyboard button with `RequestContact`), `TestContactFromSomeoneElseIsRefused` (activator not called; `bot.contact.notOwn` text), `TestOwnContactActivates` (activator called with `"telegram"`, from id, chat id, phone as sent), `TestRepliesPerResult` (each result → its locale key's text in the expected language), `TestGroupMessageGoesToSinkWithoutReply` (supergroup message in a forum topic → sink got ChatID, ThreadID, sender and text; no `SendMessage` call), `TestOtherTextGetsHelp`, `TestWebhookRejectsWrongSecret` (401, no handling), `TestWebhookHandlesUpdate` (200; activator called), `TestWebhookPathStable` (same secret → same path; differs for another secret; length 18+32), `TestPollingAdvancesOffset` (second `GetUpdates` call carries offset last+1; stops on cancel), `TestBotURL`.
- [ ] **Step 2: Run** `go test ./internal/messenger/...` — Expected: FAIL.
- [ ] **Step 3: Implement**; `go get github.com/PaulSonOfLars/gotgbot/v2@v2.0.0-rc.37`. Add locale keys `bot.start.prompt`, `bot.start.button`, `bot.contact.notOwn`, `bot.activated`, `bot.bootstrap`, `bot.alreadyConnected`, `bot.pending`, `bot.notRegistered`, `bot.rateLimited`, `bot.help` in uk and en. Add the privacy-mode note to `docs/development.md`. Add `Server.Mount` with a test `TestMountedHandlerServed` in `internal/httpapi`.
- [ ] **Step 4: Run** `go test -race ./internal/messenger/... ./internal/httpapi/ ./internal/i18n/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: Telegram adapter with polling and webhook modes`

### Task 10: HTTP auth plumbing — client IP, sessions, CSRF, admin guard

**Files:**
- Create: `internal/httpapi/auth.go`, `internal/httpapi/auth_test.go`
- Modify: `internal/config/config.go`, `internal/config/config_test.go`, `.env.example`, `docs/development.md`

**Interfaces:**
- Consumes: `domain.AuthService.Authenticate`.
- Produces:
  ```go
  package config
  // TrustProxy: take the client IP from the rightmost X-Forwarded-For entry (Railway). Env TRUST_PROXY, default false.
  Config.TrustProxy bool

  package httpapi
  const SessionCookie = "__Host-session"
  type Authenticator interface { Authenticate(ctx context.Context, raw string) (domain.User, domain.Session, error) }
  func ClientIP(r *http.Request, trustProxy bool) string
  func (s *Server) WithSession(a Authenticator) // enables the middleware below on API routes
  func CurrentUser(r *http.Request) (domain.User, bool)
  func RequireUser(h http.Handler) http.Handler      // 401 {"error":{"code":"unauthorized"}}
  func RequireAdmin(h http.Handler) http.Handler     // 401 when no user, 403 "forbidden" when not admin
  func SetSessionCookie(w http.ResponseWriter, token string, maxAge time.Duration)
  func ClearSessionCookie(w http.ResponseWriter)
  ```

CSRF backstop: every API request with method other than GET/HEAD/OPTIONS must carry `X-Requested-With: homelander` → else 403 `csrf`. Cookie: `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`, `Max-Age` = `SESSION_MAX_TTL`.

- [ ] **Step 1: Write failing tests:** `TestClientIP` (direct → RemoteAddr host; trusted proxy → rightmost XFF; untrusted → XFF ignored), `TestCSRFHeaderRequired` (POST without header → 403 `csrf`; with → passes; GET without → passes), `TestRequireUser` and `TestRequireAdmin` with a fake authenticator, `TestSessionCookieAttributes`, `TestLoadTrustProxy` in config.
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement**; document `TRUST_PROXY=true` for Railway in `.env.example` and `docs/development.md`.
- [ ] **Step 4: Run** `go test -race ./internal/httpapi/ ./internal/config/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: client IP, session middleware, CSRF backstop and admin guard`

### Task 11: Public and account API

**Files:**
- Create: `internal/httpapi/public.go`, `internal/httpapi/public_test.go`, `internal/httpapi/errors.go`, `internal/httpapi/dto.go`, `internal/httpapi/apitest_test.go`

**Interfaces:**
- Consumes: `UserService`, `CatalogService`, `AuthService`, Task 10 middleware.
- Produces: `func RegisterPublic(s *Server, d API)` where `type API struct { Users *domain.UserService; Catalog *domain.CatalogService; Auth *domain.AuthService; Messenger domain.Messenger; Cfg config.Config }`, and `func writeDomainError(w, err)`: `ValidationError` → 422 `validation` + fields; `ErrNotFound` 404; `ErrConflict` 409; `ErrForbidden` 403; `ErrLastAdmin` 409 `last_admin`; `ErrRateLimited` 429; `ErrTryLater` 503 `try_later`; `ErrInvalidCode` 401 `invalid_code`; `ErrUnauthorized` 401; else 500 `internal` (logged). JSON field names are camelCase.

Routes (all under `/api/v1`):

| Method, path | Request | Success |
|---|---|---|
| `GET /site` | — | 200 `{"communityName":{…},"welcomeText":{…},"defaultLocale":"uk","pictureUrl":"/api/v1/assets/3"\|null,"botUrl":"https://t.me/…","registerUrl":"<PUBLIC_URL>/register"}` |
| `GET /register/options` | — | 200 `{"houses":[{"id","name","apartmentFrom","apartmentTo"}],"statuses":[{"id","code","names"}]}` (not archived) |
| `POST /register` | `{"phone","firstName","lastName","houseId","apartment","statusId","consent","locale"}` | 202 `{"status":"received"}` |
| `POST /auth/code` | `{"phone"}` | 202 `{"status":"sent_if_allowed"}` |
| `POST /auth/verify` | `{"phone","code"}` | 200 `{"user":Me}` + session cookie |
| `POST /auth/logout` | — | 204, cookie cleared |
| `GET /me` | — | 200 `Me` = `{"id","firstName","lastName","phone","locale","isAdmin","isResident","residencies":[{"houseId","houseName","apartment","statusId","statusNames"}]}` |
| `GET /assets/{id}` | — | image bytes, stored content type, `Cache-Control: public, max-age=3600`, `X-Content-Type-Options: nosniff` |

Request bodies are limited to 64 KiB and decoded with `DisallowUnknownFields`; malformed JSON → 400 `bad_request`.

- [ ] **Step 1: Write failing tests** (`apitest_test.go` builds a full server over a temp DB, a fake messenger and a fixed clock): `TestSiteDefaults`, `TestRegisterOptionsHidesArchived`, `TestRegisterAccepted`, `TestRegisterValidation422`, `TestRegisterSameAnswerForKnownPhone` (status, headers except `Date`, and body byte-identical for a new and an existing phone), `TestRegisterRateLimited429`, `TestAuthCodeSameAnswer` (active, unknown, pending phones → identical 202; fake messenger got exactly one message), `TestVerifySetsCookieAndMe`, `TestVerifyWrongCode401`, `TestMeRequiresSession`, `TestLogoutClearsCookie`, `TestAssetServed`, `TestMalformedJSON400`, `TestUnknownFieldRejected`.
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/httpapi/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: public registration, login and account API`

### Task 12: Admin API

**Files:**
- Create: `internal/httpapi/admin.go`, `internal/httpapi/admin_test.go`

**Interfaces:**
- Consumes: Task 11 `API`, `RequireAdmin`.
- Produces: `func RegisterAdmin(s *Server, d API)`. All routes under `/api/v1/admin`, all behind `RequireAdmin`:

| Method, path | Body / query | Success |
|---|---|---|
| `GET /summary` | — | `{"pendingApplications":n}` |
| `GET /users` | `q, houseId, status, isAdmin, isResident, page (1…), pageSize (≤100, default 25)` | `{"items":[UserSummary],"total":n}`; `UserSummary` = User fields + `"hasTelegram"` + `"residencies"` (as in `Me`) |
| `POST /users` | `{"phone","firstName","lastName","isResident","isAdmin"}` | 201 `UserCard` |
| `GET /users/{id}` | — | `UserCard` = `{"user":…,"channels":[{"id","type","username","linkedAt"}],"residencies":[…],"audit":[{"at","action","actorUserId","details"}]}` |
| `PATCH /users/{id}` | any of `firstName, lastName, phone, locale, isResident, isAdmin` | `UserCard` |
| `POST /users/{id}/approve` · `/reject` · `/disable` · `/enable` | — | `UserCard` |
| `DELETE /users/{id}/channels/{channelId}` | — | `UserCard` |
| `POST /users/{id}/residencies` | `{"houseId","apartment","statusId"}` | 201 `UserCard` |
| `PATCH /residencies/{id}` · `DELETE /residencies/{id}` | `{"houseId","apartment","statusId"}` | `UserCard` of its user |
| `GET /houses` | `archived=true` to include archived | `[House]` |
| `POST /houses` · `PATCH /houses/{id}` | `{"name","address","latitude","longitude","floors","apartmentFrom","apartmentTo","sortOrder","archived"}` | `House` |
| `GET /houses/{id}` | — | `{"house":House,"stats":{"apartmentsTotal","apartmentsWithResidents","people","byResidencyStatus":{"<id>":n},"byUserStatus":{…},"apartments":{"<n>":people}}}` |
| `GET /houses/{id}/apartments/{n}` | — | `[{"user":UserSummary,"residency":…}]` |
| `GET /statuses` · `POST /statuses` · `PATCH /statuses/{id}` | `{"code","names","sortOrder","archived"}` | `ResidencyStatus` |
| `GET /site` · `PUT /site` | `{"communityName","welcomeText","defaultLocale"}` | admin view of site settings |
| `PUT /site/picture` (multipart field `picture`, ≤ 5 MiB + 1 KiB) · `DELETE /site/picture` | — | site settings |
| `GET /audit` | `userId, page, pageSize` | `{"items":[…],"total":n}` |

The actor of every call is `Actor{UserID: &current.ID, IP: ClientIP(...)}`.

- [ ] **Step 1: Write failing tests:** `TestAdminRoutesRequireAdmin` (every route: no session → 401, resident session → 403), `TestUsersListSearchAndPaging`, `TestApproveFlow` (apply → summary 1 → approve → user awaiting_activation → summary 0), `TestAddUserAndCard`, `TestPatchUserFlags` (revoke last admin → 409 `last_admin`), `TestUnlinkChannel`, `TestResidencyCRUD`, `TestHouseDetailStats`, `TestApartmentResidents`, `TestStatusCRUD`, `TestSitePictureUpload` (PNG accepted and served via `/assets/{id}`; SVG → 422 `picture: type`; oversize → 422 `picture: size`), `TestAuditList`.
- [ ] **Step 2: Run** — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/httpapi/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: admin API for users, houses, statuses, site and audit`

### Task 13: Wiring, background jobs and end-to-end check

**Files:**
- Modify: `cmd/homelander/main.go`
- Create: `cmd/homelander/main_test.go`

**Interfaces:**
- Consumes: everything above.

`run` builds repos, `ratelimit.New(time.Now)`, services (`PendingCap: 200`, `CodeTTL: 5*time.Minute`, `MaxAttempts: 5`, TTLs from config, `Rand: crypto/rand.Reader`), `gotgbot.NewBot(cfg.TelegramBotToken, nil)` → `telegram.New(…, telegram.NopSink{}, …)`, the HTTP server with `WithSession`, `RegisterPublic`, `RegisterAdmin`; in webhook mode `Mount(telegram.WebhookPath(cfg.AppSecret), bot.WebhookHandler(cfg.TelegramWebhookSecret))` and `StartWebhook(PUBLIC_URL + path)` after the listener is up; in polling mode `go bot.RunPolling(ctx)`. A goroutine deletes expired sessions hourly. Also apply the M0 deferred minors: `http.Server` gets `ReadTimeout: 30s`, `WriteTimeout: 60s`, `IdleTimeout: 120s`; call `stop()` after the shutdown signal so a second signal kills the process; session TTLs in `Config.LogValue` are logged as strings.

For tests, `run` takes the bot API as a parameter through an unexported `runWith(ctx, getenv, stderr, newAPI func(token string) (telegram.API, error))`.

- [ ] **Step 1: Write failing test** `TestRunEndToEnd`: start `runWith` with a fake Telegram API on `127.0.0.1:0`-style free port and `BOOTSTRAP_ADMIN_PHONE=+380500000001`; feed the fake a `/start` and an own contact for that phone; then over HTTP: `POST /api/v1/auth/code` → the fake API received a message containing a 6-digit code → `POST /api/v1/auth/verify` with it → 200, `isAdmin: true`; `POST /api/v1/admin/houses` → 201; `GET /api/v1/register/options` lists the house; cancel the context → `runWith` returns nil.
- [ ] **Step 2: Run** `go test ./cmd/homelander/` — Expected: FAIL.
- [ ] **Step 3: Implement** the wiring.
- [ ] **Step 4: Run** `make lint && make test` — Expected: PASS.
- [ ] **Step 5: Manual check with a real test bot** (owner's token in `.env`, never committed): `make run`, open the bot, `/start`, share contact from `BOOTSTRAP_ADMIN_PHONE`, log in at `http://localhost:8080/login` via `curl` (web screens come in plan 3) — the code arrives in Telegram.
- [ ] **Step 6: Commit** `feat: wire services, bot and API into the binary`
