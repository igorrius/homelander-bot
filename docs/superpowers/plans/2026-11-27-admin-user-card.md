# Admin user card (step 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the one-column user card into a summary column with the main actions plus tabs, and show the bot link to people who have not activated yet.

**Architecture:** Front-end only, in `web/src/pages/admin/UserCardPage.tsx` (split into `UserSummary.tsx` and the existing section components). No API changes.

**Tech Stack:** React 19, Mantine 9 (`Grid`, `Tabs`, `Breadcrumbs`), `@tabler/icons-react`, `qrcode.react`, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-11-27-admin-user-card-design.md` (issue #35)

## Global Constraints

- Every text in `web/src/locales/uk.json` and `en.json` with the same keys; Ukrainian default.
- Synthetic data only. Mantine only. No API changes; every existing card action keeps working.
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

- Unknown `?tab=` value: falls back to Details, no blank panel — test in Task 2.
- A nameless bootstrap admin (empty first/last name): avatar and title still render (phone as name) — test in Task 1.
- Bot link when `site.botUrl` is empty or `/site` fails: the block hides instead of showing an empty link — test in Task 1.
- Switching to another user's card keeps no tab-local edit state (existing "does not carry one user's errors" test keeps passing).
- Phone width: summary above tabs, tab list scrolls instead of overflowing — browser check in Task 3.

---

### Task 1: Summary column, bot link, breadcrumbs

**Files:**
- Create: `web/src/pages/admin/UserSummary.tsx`
- Modify: `web/src/pages/admin/UserCardPage.tsx` (header + `Access` move into the summary; layout `Grid` with summary `span={{ base: 12, md: 4 }}`, rest `span={{ base: 12, md: 8 }}`)
- Modify: locales (`admin.user.botLink.{title,hint}`, `admin.user.telegramNone`, `admin.user.roles`)
- Test: `web/src/pages/admin/UserCardPage.test.tsx`

**Interfaces:**
- Produces: `UserSummary({ card: UserCard })` — renders the avatar, `h2` with `fullName(user)`, `StatusBadge`, source badge, phone + `CopyButton` (aria-label `common.copy`), Telegram `@username` of the first `telegram` channel or `admin.user.telegramNone`, the Resident/Admin switches and status actions (moved unchanged from `Access`), dates, and — for `awaiting_activation` with a non-empty `useSite().data.botUrl` — a block titled `admin.user.botLink.title` with the hint, the URL, a copy button and a 140 px QR (`title={t('qr.title')}`).
- Breadcrumbs: `Anchor` to `/admin/users` labelled `admin.nav.users`, then the name as plain text.

- [ ] **Step 1:** Tests: an `awaiting_activation` card shows `uk.admin.user.botLink.title`, `https://t.me/test_bot` and a QR (`getByTitle(uk.qr.title)`); an `active` card does not; with `GET /site` → `{ ...site, botUrl: '' }` the block is absent; a breadcrumb link `uk.admin.nav.users` points to `/admin/users`; a card with empty first/last name shows the phone as the heading.
- [ ] **Step 2:** `npx vitest run src/pages/admin/UserCardPage.test.tsx` → the new tests FAIL.
- [ ] **Step 3:** Implement `UserSummary`; move the header and `Access` into it; wrap the page in the grid.
- [ ] **Step 4:** File passes; full `npx vitest run` green.
- [ ] **Step 5:** Commit `feat(web): user card summary column with bot link for people not yet activated`.

### Task 2: Tabs with the open tab in the URL; icons in apartments

**Files:**
- Modify: `web/src/pages/admin/UserCardPage.tsx`, locales (`admin.user.tabs.{details,apartments,channels,history}`)
- Test: `web/src/pages/admin/UserCardPage.test.tsx`

**Interfaces:**
- Tabs (`Tabs` with `keepMounted={false}`): `details` → `Details`, `apartments` → `Residencies`, `channels` → `Channels`, `history` → audit card. Tab labels for apartments and messengers append the count as a small `Badge`. The value comes from `?tab=`; unknown or missing → `details`; changing the tab sets `?tab=` with `replace: true`.
- Residency edit/remove buttons: `IconPencil` / `IconTrash` with the existing aria-labels.

- [ ] **Step 1:** Tests: `/admin/users/5` shows the Details form and not the residencies table; `/admin/users/5?tab=apartments` shows the residencies table; `?tab=bogus` shows the Details form; clicking the `uk.admin.user.tabs.history` tab makes `currentLocation()` end with `?tab=history`; the apartments tab label contains the residency count. Existing residency/channel tests open their tab first (by URL).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** File passes; full `npx vitest run` green.
- [ ] **Step 5:** Commit `feat(web): user card sections in tabs, open tab kept in the URL`.

### Task 3: Verify and PR

- [ ] **Step 1:** `make test` and `make lint` pass.
- [ ] **Step 2:** Browser on synthetic data: cards for pending, awaiting activation, active, disabled users at 1280 and 375 px, light and dark; approve from the card; copy the bot link.
- [ ] **Step 3:** Whole-branch review, fix pass, push, PR into `main` closing #35.
