# Admin polish (step 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace leftover text glyphs with icons, name people/houses/statuses in the audit log, put lists in cards with real empty states, add status tabs on Users and a theme switch, and close the step 1–2 minors.

**Architecture:** One API change (audit entry names, two more summary counters) in `internal/httpapi`; the rest is front-end in `web/src`.

**Tech Stack:** Go 1.27, React 19, Mantine 9, `@tabler/icons-react`, TanStack Query 5, Vitest.

**Spec:** `docs/superpowers/specs/2026-11-27-admin-polish-design.md` (issue #49)

## Global Constraints

- Every text in `web/src/locales/uk.json` and `en.json` with the same keys; Ukrainian default.
- Synthetic data only; Mantine only. Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

- Audit entry whose user no longer resolves (lookup fails): falls back to the raw target, no 500 — test in Task 1.
- Nameless user as audit target: shows the phone, not an empty link — test in Task 1.
- Users tabs when the summary fails: tabs still work, counts hidden — test in Task 3.
- `?status=` set by an Overview counter link selects the matching tab — test in Task 3.
- Theme choice survives a reload (stored by Mantine) — browser check in Task 5.

---

### Task 1: API — names in audit entries, more summary counters

**Files:** Modify `internal/httpapi/admin.go` (`auditDTO`, `toAudit`, `audit`, `writeCard`, `summary`); Test `internal/httpapi/admin_test.go`.

**Interfaces:**
- Produces: audit JSON adds `actorName: string`, `targetName: string`, `targetNames: {uk?,en?} | null`. `toAudit(ctx, api, es)` resolves user ids via `api.Users.Get` (memoized per call; error → empty name) and house/status ids via `newNamer`. User name = `strings.TrimSpace(first+" "+last)`, else phone.
- Summary adds `disabledUsers`, `rejectedUsers`.

- [ ] **Step 1:** Tests: after approving an application the audit list has an entry with `targetName` = applicant's full name and `actorName` = the admin's name (or phone); creating a house yields `targetName` = house name; editing a status yields `targetNames.uk`; an entry targeting `user:9999` has `targetName` "" and the request is 200; summary has `disabledUsers`/`rejectedUsers` keys.
- [ ] **Step 2:** `go test ./internal/httpapi` → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./internal/...` → PASS. TS: `AuditEntry` gains the three fields; `Summary` gains the two.
- [ ] **Step 5:** Commit `feat(api): names in audit entries; summary counts disabled and rejected users`.

### Task 2: Icons instead of glyphs, house breadcrumbs, theme switch

**Files:** Modify `web/src/pages/admin/HousePage.tsx`, `UsersPage.tsx`, `web/src/components/QrLink.tsx`, `AdminLayout.tsx` (user menu), locales (`admin.theme.{title,light,dark,auto}`); Test `HousePage.test.tsx`, `AdminLayout.test.tsx`.

- [ ] **Step 1:** Tests: house page has a breadcrumb link `uk.admin.nav.houses` → `/admin/houses` and no "←" text anywhere in `document.body.textContent`; users list shows a Telegram icon with aria-label `uk.admin.users.linked` and no "✓"; user menu → `uk.admin.theme.dark` sets `document.documentElement` attribute `data-mantine-color-scheme` to `dark`.
- [ ] **Step 2:** Run → FAIL. **Step 3:** Implement (`useMantineColorScheme().setColorScheme`; checked item shows `IconCheck`). **Step 4:** Suite green.
- [ ] **Step 5:** Commit `feat(web): icons instead of text glyphs; theme switch in the user menu`.

### Task 3: Lists in cards, empty states, Users status tabs, statuses without code column

**Files:** Create `web/src/components/EmptyState.tsx`; Modify `UsersPage.tsx`, `HousesPage.tsx`, `StatusesPage.tsx`, `AuditPage.tsx`, `ApplicationsPage.tsx`, `OverviewPage.tsx`, locales (`admin.users.tabs.all`, `admin.houses.emptyHint`, `admin.applications.emptyHint`); Tests for each page touched.

**Interfaces:** `EmptyState({ icon: Icon; title: string; hint?: string; action?: ReactNode })`. Users tabs: `Tabs variant="pills"` values `all | pending_approval | awaiting_activation | active | disabled | rejected`, label `userStatus.<s>` plus count badge from `useSummary()` (`pendingApplications`, `awaitingActivation`, `activeUsers`, `disabledUsers`, `rejectedUsers`); selecting sets `?status=` (cleared for `all`) and page 1; the status `Select` is removed.

- [ ] **Step 1:** Tests: `/admin/users?status=awaiting_activation` has the tab `uk.userStatus.awaiting_activation` selected (`aria-selected="true"`) and requests `status=awaiting_activation`; clicking tab `uk.userStatus.disabled` requests `status=disabled`; tab labels show counts from the summary; with summary 500 the tabs still render; houses empty state shows `uk.admin.houses.empty` and an `uk.admin.houses.add` button; statuses list has no `uk.admin.statuses.code` column header (form still has the code field).
- [ ] **Step 2:** Run → FAIL. **Step 3:** Implement. **Step 4:** Suite green.
- [ ] **Step 5:** Commit `feat(web): lists in cards, empty states, status tabs on Users`.

### Task 4: Audit names in the table; step 1–2 minors

**Files:** Modify `AuditTable.tsx`, `ApplicationList.tsx`, `UserSummary.tsx`, `ApplicationsPage.tsx`, `components/InviteLink.tsx`, locales (`admin.user.copyPhone`, `admin.user.copyLink`, `admin.invite.downloadFailed`; remove `admin.applications.appliedAt`, `admin.user.access`); Tests `AuditPage.test.tsx`, `UserCardPage.test.tsx`, `ApplicationsPage.test.tsx`, `InviteLink.test.tsx`.

- [ ] **Step 1:** Tests: audit row shows link "Olena Test" → `/admin/users/5` and actor name link; a house target shows its name linked to `/admin/houses/2`; a status target shows the localized name linked to `/admin/statuses`; an entry without names shows the raw target; user summary has buttons named `uk.admin.user.copyPhone` and (awaiting) `uk.admin.user.copyLink`; Applications "Registration link" with `/site` 500 opens a dialog with the error text; InviteLink shows `uk.admin.invite.downloadFailed` when the image fails to load (stub `Image` calling `onerror`).
- [ ] **Step 2:** Run → FAIL. **Step 3:** Implement; `ApplicationList` name gets `overflowWrap: 'anywhere'`. **Step 4:** Suite green.
- [ ] **Step 5:** Commit `feat(web): names in the audit log; small fixes from steps 1–2`.

### Task 5: Verify and PR

- [ ] `make test`, `make lint`; browser on synthetic data (1280 / 375, light / dark, theme switch survives reload); whole-branch review; fix pass; PR into `main` closing #49.
