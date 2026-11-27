# Admin navigation, overview and visual base (step 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the admin panel a grouped menu with icons, a theme, an Overview home page with the registration link/QR and the most-used actions, and an applications list that works on a phone.

**Architecture:** Front-end work in `web/src/pages/admin` and `web/src/components` on the existing Mantine 9 stack, plus two extra counters in the existing `GET /api/v1/admin/summary` handler. No new endpoints, no migrations.

**Tech Stack:** Go 1.27, React 19, Mantine 9, TanStack Query 5, react-i18next, `@tabler/icons-react` (new, MIT), `qrcode.react`, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-11-27-admin-navigation-and-overview-design.md` (issue #34)

## Global Constraints

- Every user-facing text in `web/src/locales/uk.json` and `en.json` with the same keys (parity test). Ukrainian is the default.
- Synthetic data only (`+380500000000`-style phones, "House A", "Test Community").
- Mantine stays (ADR-0001); no Tailwind, no other component library.
- No admin function is removed; only the registration link moves (Site → Overview + Applications modal).
- Docs, comments, commits in English; commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

- First run with no houses/statuses: Overview shows the checklist and still shows the registration link (it works once setup is done) — test in Task 4.
- Approve/Reject on a 375 px screen: buttons wrap under the name, no table — covered by the list markup in Task 3 (checked in the browser in Task 5).
- Long names/phones on narrow screens: the list row wraps instead of overflowing — browser check in Task 5.
- Summary request fails: Overview still renders the invite card and the counters show "—" rather than crashing — test in Task 4.
- Download QR in a browser without canvas support (jsdom): the button must not throw — test in Task 3.

---

### Task 1: Summary counters (backend + type)

**Files:**
- Modify: `internal/httpapi/admin.go` (`summary` handler)
- Modify: `internal/domain/users_service.go` (add `CountByStatus` passthrough)
- Test: `internal/httpapi/admin_test.go` (`TestApproveFlow`)
- Modify: `web/src/api/admin.ts` (`useSummary` type), `web/src/api/types.ts` (`Summary`)

**Interfaces:**
- Produces: JSON `{"pendingApplications": n, "awaitingActivation": n, "activeUsers": n}`; TS `interface Summary { pendingApplications: number; awaitingActivation: number; activeUsers: number }`; `useSummary(): UseQueryResult<Summary>`.

- [ ] **Step 1:** In `TestApproveFlow`, after the approve call assert the summary has `pendingApplications == 0`, `awaitingActivation == 1`, `activeUsers == 1` (the admin from `adminEnv`).
- [ ] **Step 2:** `go test ./internal/httpapi -run TestApproveFlow` → FAIL (keys missing).
- [ ] **Step 3:** Add `func (s *UserService) CountByStatus(ctx context.Context, st UserStatus) (int, error)`; the handler fills all three keys from it (keep `PendingCount` for its existing callers).
- [ ] **Step 4:** Test passes; update `Summary` type and `useSummary`; `adminRoutes` fixture summary becomes `{ pendingApplications: 0, awaitingActivation: 0, activeUsers: 1 }`.
- [ ] **Step 5:** Commit `feat(api): admin summary counts awaiting activation and active users`.

### Task 2: Theme, icons, page header, grouped menu and user menu

**Files:**
- Modify: `web/package.json` (add `@tabler/icons-react`)
- Create: `web/src/theme.ts`; Modify: `web/src/Providers.tsx`
- Create: `web/src/components/PageHeader.tsx`
- Modify: `web/src/pages/admin/AdminLayout.tsx`, every admin page title (`ApplicationsPage`, `UsersPage`, `UserCardPage`, `HousesPage`, `HousePage`, `StatusesPage`, `SitePage`, `AuditPage`)
- Modify: locales (`admin.navGroup.{people,community,settings}`, `admin.account`)
- Test: `web/src/pages/admin/AdminLayout.test.tsx`

**Interfaces:**
- Produces: `theme` (Mantine `createTheme`: `primaryColor: 'teal'`, `defaultRadius: 'md'`, `Card` default `withBorder` + `radius: 'md'`, system font stack); `PageHeader({ icon?: TablerIcon; title: string; description?: ReactNode; actions?: ReactNode })` rendering an `h2` with the title; nav model in `AdminLayout`: `navGroups: { label?: string; items: { to: string; icon: TablerIcon; label: string; badge?: number }[] }[]`.
- Icons: Applications `IconInbox`, Users `IconUsers`, Houses `IconBuildingCommunity`, Site `IconWorld`, Statuses `IconTags`, Audit `IconHistory`, Overview `IconLayoutDashboard` (added in Task 4).

- [ ] **Step 1:** Tests: the menu shows the group labels `uk.admin.navGroup.people`, `.community`, `.settings`; the user menu button (accessible name = admin's full name) opens items linking to `/` (`uk.admin.residentPage`) and a `uk.home.logout` item that calls `POST /auth/logout`. Keep the pending-counter test.
- [ ] **Step 2:** Run `cd web && npx vitest run src/pages/admin/AdminLayout.test.tsx` → FAIL.
- [ ] **Step 3:** `npm i @tabler/icons-react`; add theme; rebuild the navbar from `navGroups` (group label = small dimmed uppercase `Text`, `NavLink` with `leftSection` icon); header: app icon `/favicon.svg` + title, `LanguageSwitcher`, Mantine `Menu` user menu. Replace each page's `Title order={2}` with `PageHeader` (keep the same title text so existing tests stay green; existing right-side buttons go into `actions`).
- [ ] **Step 4:** `npx vitest run` → all green.
- [ ] **Step 5:** Commit `feat(web): admin theme, icons, grouped menu and user menu`.

### Task 3: Invite block, phone-friendly applications list, link out of Site

**Files:**
- Create: `web/src/components/InviteLink.tsx`
- Create: `web/src/pages/admin/ApplicationList.tsx`
- Modify: `web/src/pages/admin/ApplicationsPage.tsx`, `web/src/pages/admin/SitePage.tsx`, locales
- Test: `web/src/pages/admin/ApplicationsPage.test.tsx`, `web/src/pages/admin/SitePage.test.tsx`, `web/src/components/InviteLink.test.tsx`

**Interfaces:**
- Produces: `InviteLink({ url: string })` — QR (SVG, 180 px), the link, Copy button, "Download QR" button that renders a hidden 1024 px `QRCodeCanvas` and saves `registration-qr.png` (no-op if `toDataURL` is unavailable), one-line hint `admin.invite.hint`. `ApplicationList({ users: AdminUser[] })` — one bordered row per user: name link to `/admin/users/:id`, phone, apartments, applied date; Approve / Reject (with confirm) on the right, wrapping under the details on narrow screens (`Group justify="space-between" wrap="wrap"`); per-row error text as today.
- Locale keys: `admin.invite.{title,hint,download,open}`; `admin.site.registerLink*` keys removed.

- [ ] **Step 1:** Tests: ApplicationsPage — existing four tests still pass against the list (no `table` role present: `expect(screen.queryByRole('table')).toBeNull()`); a `uk.admin.invite.open` button opens a dialog showing `http://localhost/register`. SitePage — `http://localhost/register` is no longer on the page. InviteLink — clicking `uk.admin.invite.download` in jsdom does not throw.
- [ ] **Step 2:** Run the three test files → FAIL.
- [ ] **Step 3:** Implement `InviteLink`, `ApplicationList`; ApplicationsPage uses `ApplicationList` and a header action button opening a `Modal` with `InviteLink` (url from `useSite().data.registerUrl`); remove the registration card from SitePage (and its now-unused imports; `QrLink` stays for the public pages).
- [ ] **Step 4:** Tests green.
- [ ] **Step 5:** Commit `feat(web): invite block with QR download and a phone-friendly applications list`.

### Task 4: Overview page

**Files:**
- Create: `web/src/pages/admin/OverviewPage.tsx`, `web/src/pages/admin/OverviewPage.test.tsx`
- Modify: `web/src/App.tsx` (index route), `web/src/pages/admin/AdminLayout.tsx` (Overview nav item, active only on exact `/admin`), `web/src/test/fixtures.ts` (`adminRoutes` gets `GET /admin/houses` → `houses`, `GET /admin/statuses` → `statuses`), locales (`admin.nav.overview`, `admin.overview.*`)
- Modify: `web/src/pages/admin/AdminLayout.test.tsx` ("opens the applications queue by default" → "opens the overview by default")

**Interfaces:**
- Consumes: `useSummary` (Task 1), `useHouses()`, `useStatuses()`, `useUsers({ status: 'pending_approval', pageSize: 5 })`, `InviteLink`, `ApplicationList` (Task 3), `PageHeader` (Task 2), `useSite()`.
- Produces: route `/admin` → `OverviewPage`.
- Layout: checklist (only when no non-archived house or no non-archived status; items `admin.overview.setup.{house,status,community}` linking to `/admin/houses`, `/admin/statuses`, `/admin/site`; the community item is shown when the site's community name is empty in every locale); four stat cards (pending → `/admin/applications`, awaiting → `/admin/users?status=awaiting_activation`, active → `/admin/users?status=active`, houses → `/admin/houses`) showing `—` while loading or on error; invite card; latest applications card with `ApplicationList`, an empty text `admin.applications.empty`, and an "All applications" link.

- [ ] **Step 1:** Tests: `/admin` shows heading `uk.admin.overview.title`; counters show `2`, `3`, `7`, `2` from a summary `{2,3,7}` and the two fixture houses, and the pending card links to `/admin/applications`; the register URL is shown; with `GET /admin/houses` → `[]` the checklist item `uk.admin.overview.setup.house` is a link to `/admin/houses`, and with both lists non-empty it is absent; a pending user appears with an Approve button that posts to `/admin/users/5/approve`; summary answering 500 still shows the register URL and `—`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement; switch the index route from `<Navigate to="applications">` to `<OverviewPage />`.
- [ ] **Step 4:** `npx vitest run` → all green.
- [ ] **Step 5:** Commit `feat(web): admin overview with invite link, counters and latest applications`.

### Task 5: Verify, docs, PR

**Files:**
- Modify: `docs/development.md` ("First run" step 4: the registration link is on Admin → Overview)

- [ ] **Step 1:** `make test` and `make lint` → both pass.
- [ ] **Step 2:** `make web`, run the local server, check `/admin`, `/admin/applications`, `/admin/site` at 1280 px and 375 px in uk and en, light and dark; approve one application from the Overview on 375 px.
- [ ] **Step 3:** Update `docs/development.md`; commit `docs: registration link lives on the admin overview`.
- [ ] **Step 4:** Push the branch, open a PR into `main` closing #34, with screenshots described in text.
