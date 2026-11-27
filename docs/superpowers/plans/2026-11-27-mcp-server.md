# MCP server (part 3 of 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An admin with the new **MCP** flag connects an AI agent (Claude) to Homelander through OAuth with our normal phone + Telegram-code login; the agent then reads and writes the data space and writes home-page drafts through MCP tools, never publishes, and everything it does is audited as "*admin* via MCP (*client*)".

**Architecture:** Homelander is its own OAuth 2.1 authorization server (PKCE S256, dynamic client registration restricted to allow-listed redirect URIs) and an MCP resource server at `/mcp` (Streamable HTTP, official Go SDK `github.com/modelcontextprotocol/go-sdk`). OAuth state lives in SQLite (migration `0006`); an `McpAuthService` in `internal/domain` issues and verifies hashed tokens bound to a *grant* (one user + one client = one "connected agent"). MCP tools are thin adapters over the existing `DataService` and `PageService`, called with an `Actor` that carries `Via: "mcp"` and the client name. The consent screen is a page of the React app.

**Tech Stack:** Go, `github.com/modelcontextprotocol/go-sdk` (`mcp`, `auth`, `oauthex`), SQLite, React + Mantine, vitest.

**Spec:** [docs/superpowers/specs/2026-11-27-site-data-pages-and-mcp-design.md](../specs/2026-11-27-site-data-pages-and-mcp-design.md) — sections 2, 6, 7, 9 (step 3). Builds on part 1 (merged, PR #53) and part 2 (`feat/site-pages`, PR #56): this branch starts from `feat/site-pages`.

## Global Constraints

- New migration `internal/storage/sqlite/migrations/0006_mcp.sql`; earlier ones untouched.
- MCP is effective only when the user is **active AND Admin AND MCP**; checked on consent, on token exchange/refresh and on **every** MCP request. Removing Admin keeps `is_mcp` stored but ineffective (UI shows it off and disabled).
- Any admin may switch MCP on/off for any admin, including themselves. Audit `user.mcp_granted` / `user.mcp_revoked`.
- OAuth: authorization code + PKCE **S256 only**; public clients (`token_endpoint_auth_method: none`); code TTL **60 s**, single use; access token TTL **1 h**; refresh token TTL **30 days**, rotated on use (old one revoked); tokens are 32 random bytes base64url, stored only as SHA-256.
- Redirect URIs are allow-listed. Config env `MCP_REDIRECT_URIS` (comma-separated). Default list: `https://claude.ai/api/mcp/auth_callback`, `https://claude.com/api/mcp/auth_callback`, `http://localhost:*`, `http://127.0.0.1:*` (`*` = any port, allowed only for loopback hosts; path free for loopback). The builder confirms the Claude callback URLs against Claude's current connector docs and records it in the ledger. Registration with any other redirect → `400 {"error":"invalid_redirect_uri"}`.
- Consent screen shows the client's self-reported name **and the redirect host**.
- Audit for actions through MCP: same actions as the panel, `ActorUserID` = the admin, details gain `"via": "mcp", "client": "<client name>"`. New actions: `mcp.client_authorized`, `mcp.client_revoked` (target `user:<id>`).
- Agent never publishes: there is no publish/unpublish/delete-page tool. `page_create_draft` returns `<PublicURL>/admin/pages/<id>`, never a preview token URL.
- Rate limit **300 MCP requests per minute per grant** → JSON-RPC/HTTP 429.
- Tool names, descriptions and error texts in English. All new UI text via i18n (uk default, en). No real addresses or phones in fixtures or the page guide.

## Review Focus

1. A token keeps working after its admin loses Admin or MCP, is disabled, or the grant is revoked → must fail on the next request with 401 (tests in Tasks 2 and 6).
2. Code replay (second exchange of the same code), wrong `code_verifier`, a `redirect_uri` different from the one authorized, an expired code → `invalid_grant`, and a replayed code revokes nothing else but stays unusable (test in Task 2).
3. Redirect URI tricks: `https://claude.ai.evil.com/api/mcp/auth_callback`, `https://claude.ai/api/mcp/auth_callback/../x`, `http://localhost.evil.com:8080/cb`, `https://localhost:8080/cb` (non-http loopback is fine, other hosts with `*` are not) → rejected unless exactly matching (test in Task 3).
4. A non-admin or an admin without MCP who opens the consent link sees why and cannot approve; the API refuses approve for them too (tests in Tasks 3 and 4).
5. Agent output that fails validation (bad key, bad page path, missing `index.html`) comes back as a readable tool error (`isError: true` with the field reasons), not a protocol failure (test in Task 6/7).

---

### Task 1: MCP flag on users

**Files:**
- Create: `internal/storage/sqlite/migrations/0006_mcp.sql` (this task adds `ALTER TABLE users ADD COLUMN is_mcp INTEGER NOT NULL DEFAULT 0 CHECK (is_mcp IN (0,1))`; Task 2 appends its tables to the same file before the branch is pushed)
- Modify: `internal/domain/user.go` (`User.IsMCP bool`), `internal/domain/users_service.go` (`UserPatch.IsMCP *bool`; `func (u User) MCPEnabled() bool` = active && IsAdmin && IsMCP), `internal/domain/audit.go` (`ActUserMCPGranted = "user.mcp_granted"`, `ActUserMCPRevoked = "user.mcp_revoked"`), `internal/storage/sqlite/users.go`, `internal/httpapi/admin.go` + `dto.go` (`isMcp` in user card and patch body), tests in `internal/domain/users_service_test.go`, `internal/httpapi/admin_test.go`

- [ ] **Step 1: Write failing tests** `TestMCPFlag` (patch `IsMCP=true` on an admin → stored, audit `user.mcp_granted`; on a non-admin → `isMcp: invalid` validation; revoking Admin keeps `IsMCP` but `MCPEnabled()` false; disabled admin → `MCPEnabled()` false), `TestAdminPatchMCP` (HTTP `PATCH /api/v1/admin/users/{id}` with `{"isMcp":true}` → card shows `isMcp: true`).
- [ ] **Step 2: Run** `go test ./internal/... -run MCP` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): MCP flag for admins`.

### Task 2: OAuth storage and McpAuthService

**Files:**
- Modify: `0006_mcp.sql` (append tables), `internal/domain/users_service.go` (`Repos.MCP`), `internal/storage/sqlite/repos.go`
- Create: `internal/domain/mcp.go` (types + `MCPRepo`), `internal/domain/mcp_auth_service.go`, `internal/domain/mcp_auth_service_test.go`, `internal/storage/sqlite/mcp.go`, `internal/storage/sqlite/mcp_test.go`

**Interfaces:**
- Schema: `mcp_clients(id TEXT PK, name TEXT, redirect_uris TEXT json, created_at)`; `mcp_grants(id INTEGER PK, user_id REFERENCES users ON DELETE CASCADE, client_id REFERENCES mcp_clients ON DELETE CASCADE, created_at, last_used_at, revoked_at, UNIQUE(user_id, client_id))`; `mcp_codes(hash TEXT PK, grant_id REFERENCES mcp_grants ON DELETE CASCADE, redirect_uri, challenge, resource, expires_at, used_at)`; `mcp_tokens(hash TEXT PK, grant_id REFERENCES mcp_grants ON DELETE CASCADE, kind TEXT CHECK IN ('access','refresh'), expires_at, revoked_at)`.
- Produces:
  ```go
  type MCPClient struct { ID, Name string; RedirectURIs []string; CreatedAt time.Time }
  type MCPGrant struct { ID, UserID int64; ClientID, ClientName string; CreatedAt time.Time; LastUsedAt, RevokedAt *time.Time }
  type MCPAuthRequest struct { ClientID, RedirectURI, Challenge, Method, State, Resource string }
  type MCPTokens struct { Access, Refresh string; ExpiresIn int }
  type MCPPrincipal struct { User User; Grant MCPGrant }
  type McpAuthService struct { R Repos; Now Clock; Allowed RedirectPolicy }
  type RedirectPolicy []string // patterns from config
  func (p RedirectPolicy) Allows(uri string) bool
  func (s *McpAuthService) Register(ctx context.Context, name string, redirectURIs []string) (MCPClient, error) // ErrInvalidRedirect
  func (s *McpAuthService) Client(ctx context.Context, id string) (MCPClient, error)
  func (s *McpAuthService) Approve(ctx context.Context, u User, req MCPAuthRequest, a Actor) (code string, err error) // ErrForbidden unless u.MCPEnabled(); audit mcp.client_authorized (once per new grant)
  func (s *McpAuthService) Exchange(ctx context.Context, code, clientID, redirectURI, verifier string) (MCPTokens, error) // ErrInvalidGrant
  func (s *McpAuthService) Refresh(ctx context.Context, refresh, clientID string) (MCPTokens, error) // rotates
  func (s *McpAuthService) Verify(ctx context.Context, access string) (MCPPrincipal, error) // ErrUnauthorized; updates last_used_at at most once a minute
  func (s *McpAuthService) Grants(ctx context.Context, userID int64) ([]MCPGrant, error) // not revoked
  func (s *McpAuthService) Revoke(ctx context.Context, grantID int64, a Actor) error   // revokes the grant and all its tokens; audit mcp.client_revoked
  var ErrInvalidGrant = errors.New("invalid_grant"); var ErrInvalidRedirect = errors.New("invalid_redirect_uri")
  ```
- `Approve` reuses the `(user, client)` grant: a revoked one is reactivated (`revoked_at` cleared). `mcp.client_authorized` is written when the grant is new or was revoked.
- `RedirectPolicy.Allows`: parse with `net/url`; exact string match for non-wildcard patterns; for `http://localhost:*` / `http://127.0.0.1:*` require scheme `http`, `Hostname()` exactly that host, any port, any path, no userinfo.

- [ ] **Step 1: Write failing tests**: repo round-trip (`mcp_test.go`); `TestRedirectPolicy` (Review Focus 3 cases plus the accepted defaults); `TestMCPAuthFlow` (register → approve → exchange with right verifier → verify returns the user → refresh rotates: old refresh `invalid_grant`, old access still valid until expiry); `TestMCPCodeRules` (Review Focus 2 cases, expired after 61 s); `TestMCPVerifyChecksUser` (Review Focus 1: revoke Admin / MCP / disable user / revoke grant → `ErrUnauthorized`; access token after 1 h → `ErrUnauthorized`); `TestMCPApproveRequiresFlag` (non-admin and admin-without-MCP → `ErrForbidden`).
- [ ] **Step 2: Run** `go test ./internal/... -run 'MCP|Redirect'` — Expected: FAIL.
- [ ] **Step 3: Implement.** PKCE: `base64url(sha256(verifier)) == challenge`, constant-time compare.
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): OAuth clients, grants, codes and tokens`.

### Task 3: OAuth HTTP endpoints and config

**Files:**
- Modify: `internal/config/config.go` (`MCPRedirectURIs []string` from `MCP_REDIRECT_URIS`, default list above), `internal/config/config_test.go`, `cmd/homelander/main.go`, `internal/httpapi/public.go` (`API.MCPAuth *domain.McpAuthService`), `docs/development.md` / `docs/deploy-railway.md` (env var)
- Create: `internal/httpapi/oauth.go`, `internal/httpapi/oauth_test.go`

**Interfaces:**
- Produces (mounted with `Server.Mount`, outside `/api/`, no CSRF header needed, `Access-Control-Allow-Origin: *` on all of them):
  - `GET /.well-known/oauth-protected-resource` and `/.well-known/oauth-protected-resource/mcp` → `auth.ProtectedResourceMetadataHandler` with `Resource: <PublicURL>/mcp`, `AuthorizationServers: [<PublicURL>]`.
  - `GET /.well-known/oauth-authorization-server` → `{issuer, authorization_endpoint: <PublicURL>/oauth/authorize, token_endpoint: <PublicURL>/oauth/token, registration_endpoint: <PublicURL>/oauth/register, revocation_endpoint: <PublicURL>/oauth/revoke, response_types_supported: ["code"], grant_types_supported: ["authorization_code","refresh_token"], code_challenge_methods_supported: ["S256"], token_endpoint_auth_methods_supported: ["none"]}`.
  - `POST /oauth/register` (RFC 7591 JSON) → 201 `{client_id, client_name, redirect_uris, token_endpoint_auth_method: "none", grant_types, response_types}`; errors per RFC 7591.
  - `POST /oauth/token` (form) → `{access_token, token_type: "Bearer", expires_in: 3600, refresh_token}`; errors `{"error": "invalid_grant" | "invalid_request" | "unsupported_grant_type"}` with 400; `Cache-Control: no-store`.
  - `POST /oauth/revoke` (form `token`) → 200 always (RFC 7009).
  - `GET /oauth/authorize` is **not** a Go route: the SPA serves it (Task 4).
  - `GET /api/v1/oauth/authorize-info?client_id&redirect_uri` (session optional) → `{clientName, redirectHost, signedIn, allowed}`; unknown client or redirect not registered for it → 400 `invalid_request`.
  - `POST /api/v1/oauth/authorize` (session + CSRF as other API calls) body `{clientId, redirectUri, codeChallenge, codeChallengeMethod, state, resource, approve}` → `{redirect: "<redirectUri>?code=…&state=…"}` or, when `approve:false`, `{redirect: "<redirectUri>?error=access_denied&state=…"}`; not allowed → 403.
  - Rate limit `/oauth/register` and `/oauth/token`: 30 per minute per IP.

- [ ] **Step 1: Write failing tests** `TestOAuthMetadata`, `TestOAuthRegister` (allowed / rejected redirects incl. Review Focus 3), `TestOAuthFullFlow` (register → authorize-info → login admin with MCP → authorize → token → refresh), `TestOAuthAuthorizeForbidden` (admin without MCP → 403; guest → 401), `TestConfigMCPRedirects` (default list; env override).
- [ ] **Step 2: Run** `go test ./internal/... -run 'OAuth|MCPRedirects'` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): OAuth endpoints with allow-listed redirects`.

### Task 4: Web — consent page, MCP switch, connected agents

**Files:**
- Create: `web/src/pages/OAuthAuthorizePage.tsx`, `web/src/pages/OAuthAuthorizePage.test.tsx`, `web/src/api/oauth.ts`
- Modify: `web/src/App.tsx` (route `/oauth/authorize`), `web/src/api/types.ts` (`AdminUser.isMcp`, `UserCard.mcpGrants: {id, clientName, createdAt, lastUsedAt}[]`), `web/src/pages/admin/UserSummary.tsx` (MCP switch next to Admin, disabled and off when not admin), `web/src/pages/admin/UserCardPage.tsx` ("Connected agents" block with Revoke + confirm), `web/src/api/admin.ts` (`useRevokeGrant`), locales; backend for the card: `internal/httpapi/admin.go` card DTO gains `mcpGrants`, route `DELETE /api/v1/admin/users/{id}/mcp-grants/{grantId}` → `McpAuthService.Revoke` (test in `oauth_test.go`).
- Consent page: reads query params; guest → login form flow (reuse `/login?next=<current url>`; add `next` support to `LoginPage` if missing, only same-origin paths); signed in but not allowed → explanation (not admin / MCP off) and only "Cancel"; allowed → "Allow *clientName* (*redirectHost*) to act as you in Homelander" with **Allow** / **Cancel**; both buttons navigate to the returned `redirect` via `window.location.assign`.

- [ ] **Step 1: Write failing tests**: consent page — guest sees login link with `next`; allowed admin sees client name and redirect host, Allow posts `approve:true` and assigns the redirect (spy on `window.location.assign`); not-allowed admin sees the reason and no Allow button. User card — MCP switch hidden-disabled for non-admin, toggling posts `{isMcp:true}`; connected agent revoke (confirm) calls the DELETE route.
- [ ] **Step 2: Run** `cd web && npx vitest run OAuthAuthorize UserCard` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): MCP consent page, MCP switch and connected agents`.

### Task 5: Actor via MCP in audit and page authorship

**Files:**
- Modify: `internal/domain/users_service.go` (`Actor.Via, Actor.Client string`; `func (a Actor) AuditDetails(d map[string]any) map[string]any` adds `via`/`client` when `Via != ""`), `internal/domain/data_service.go` and `page_service.go` (their `audit` helpers use it), `0006_mcp.sql` (`ALTER TABLE page_versions ADD COLUMN created_via TEXT NOT NULL DEFAULT ''`), `internal/domain/page.go` (`PageVersion.CreatedVia`), `internal/storage/sqlite/pages.go`, `internal/httpapi/pages.go` (DTO `createdVia`), web `AuditTable.tsx` (shows "*name* · via MCP (*client*)" from details), `PagesPage.tsx` (badge "MCP" when `createdVia`), locales.

- [ ] **Step 1: Write failing tests**: `TestAuditViaMCP` (DataService.Create and PageService.CreateDraft with `Actor{Via:"mcp", Client:"Claude"}` → audit details carry both; page `CreatedVia == "Claude"`); web `AuditPage.test.tsx` row with details `{via:'mcp', client:'Claude'}` shows the "via MCP (Claude)" text; `PagesPage.test.tsx` shows the badge.
- [ ] **Step 2: Run** `go test ./internal/domain -run ViaMCP && cd web && npx vitest run Audit PagesPage` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `make test` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): audit and page versions record actions via MCP`.

### Task 6: MCP endpoint and data tools

**Files:**
- Create: `internal/mcpserver/server.go` (`func New(api Deps) http.Handler`), `internal/mcpserver/data_tools.go`, `internal/mcpserver/server_test.go`
- Modify: `go.mod`/`go.sum` (`go get github.com/modelcontextprotocol/go-sdk@latest`), `cmd/homelander/main.go` (`server.Mount("/mcp", mcpserver.New(...))`)

**Interfaces:**
- `type Deps struct { Auth *domain.McpAuthService; Data *domain.DataService; Pages *domain.PageService; Catalog *domain.CatalogService; Limits *ratelimit.Limiter; PublicURL string; Now domain.Clock }`
- `New` builds one `mcp.Server` (name `homelander`, `Instructions` = short text telling the agent to call `page_guide` before writing a page), wraps `mcp.NewStreamableHTTPHandler` with `auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{ResourceMetadataURL: PublicURL + "/.well-known/oauth-protected-resource/mcp"})`. The verifier calls `McpAuthService.Verify` and returns `TokenInfo{UserID: strconv(user id), Expiration, Extra: {"principal": MCPPrincipal}}`; the rate limit (key `mcp:<grant id>`) runs in a wrapper after auth.
- Tools get the principal from `req.Extra.TokenInfo.Extra["principal"]` and build `domain.Actor{UserID, IP: "", Via: "mcp", Client: grant.ClientName}`.
- Data tools (typed `mcp.AddTool`, JSON field names snake_case):
  - `data_catalog()` → `{items: [{key, kind, value_type?, columns?, public, description, row_count}]}`
  - `data_get({key, offset?, limit? ≤ 200})` → value or table page `{item, rows, total}`
  - `data_set_value({key, value_type?, value, description?, public?})` → creates the value item when missing (then `value_type` required), else patches it
  - `data_create_table({key, columns, description?, public?})`, `data_update_table_columns({key, columns})`
  - `data_rows_upsert({key, rows: [{id?, position?, cells}]})` → `{rows}`; `data_rows_delete({key, ids})`
  - `data_set_public({key, public})`, `data_delete({key})`
  - `asset_upload({filename, base64})` → `{id, url}` via `CatalogService.UploadImage`
- Domain errors → `CallToolResult{IsError: true}` with text `"validation failed: key: invalid, ..."`, `"not found"`, `"conflict"`.

- [ ] **Step 1: Write failing tests** in `server_test.go` using the SDK client (`mcp.NewClient` + `mcp.StreamableClientTransport` with an `http.Client` whose transport adds `Authorization: Bearer <token>` from a real `McpAuthService` flow against an `httptest.Server`): no token → 401 with `WWW-Authenticate` naming the resource metadata URL; `tools/list` contains every data tool; `data_set_value` creates `house.address` and the audit entry has `via: mcp`; `data_set_value` with key `Bad Key` → `IsError` with `key: invalid`; token of an admin whose MCP was revoked after issue → 401 (Review Focus 1); 301st call in a minute → 429.
- [ ] **Step 2: Run** `go test ./internal/mcpserver` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): MCP endpoint with bearer auth and data tools`.

### Task 7: Page tools, the page guide, connection docs

**Files:**
- Create: `internal/mcpserver/page_tools.go`, `internal/mcpserver/guide.md` (embedded with `go:embed`), `docs/mcp.md`
- Modify: `internal/mcpserver/server.go`, `internal/mcpserver/server_test.go`, `README.md` (link to `docs/mcp.md`)

**Interfaces:**
- `page_guide()` → the guide text; the same text is also an MCP resource `homelander://guide/page` (`text/markdown`).
- `page_list_versions()` → `{versions: [{id, status, note, created_at, created_via, files: [{path, size}]}]}`
- `page_get_version({id})` → files with `text` content for html/css/js/json/svg, size only for binaries
- `page_create_draft({note?, files: [{path, text? , base64?}]})` → `{id, admin_url}`; exactly one of `text`/`base64` per file, else tool error
- `page_update_draft({id, put?: [...same...], delete?: [paths]})` → `{id, admin_url}`
- `page_copy_to_draft({id})` → `{id, admin_url}`
- `admin_url` = `<PublicURL>/admin/pages/<id>`. No publish, unpublish or delete tools.
- `guide.md` covers: the sandbox (no cookies, no same-origin API with a session, no external resources — the CSP from part 2 verbatim), file rules and limits, `index.html` required; reading data with `fetch('/api/v1/public/data')` and `/api/v1/public/data/<key>` with the exact response shapes from part 1 and image URLs; ready snippets for the `homelander:height` message (send on load and on resize via `ResizeObserver`) and handling `?lang=` + `homelander:lang`; links to `/login` and `/register` need `target="_top"`; publishing is done by an admin in the panel. Example data keys are invented (`house.address`, `contacts`), no real values.
- `docs/mcp.md`: for admins — turn on MCP on your user card; in Claude add a custom connector with URL `<PUBLIC_URL>/mcp`; sign in and approve; how to revoke; `MCP_REDIRECT_URIS` for other clients.

- [ ] **Step 1: Write failing tests**: `tools/list` contains every page tool and **no** tool whose name contains `publish` or `delete_page`; `page_create_draft` with `index.html` text + `img/a.png` base64 → version exists as draft with `created_via` = client, result `admin_url` ends with `/admin/pages/<id>` and contains no `/site/preview/`; missing `index.html` → `IsError` with `files: index_required`; `page_update_draft` on a published version → `IsError` "conflict"; `page_guide` text mentions `/api/v1/public/data` and `homelander:height`; the resource is listed.
- [ ] **Step 2: Run** `go test ./internal/mcpserver -run Page` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `make test && make lint` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(mcp): page tools, page guide and connection docs`.
