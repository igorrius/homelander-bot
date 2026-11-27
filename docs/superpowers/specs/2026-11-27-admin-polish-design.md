# Admin polish: icons, names in the audit log, lists, status tabs, theme (step 3) — design

- **Date:** 2026-11-27
- **Status:** Approved by the owner (2026-11-27)
- **Issue:** #49 · **Builds on:** [step 1](2026-11-27-admin-navigation-and-overview-design.md), [step 2](2026-11-27-admin-user-card-design.md)

## Intent

Steps 1 and 2 gave the admin a visual base, an overview and a new user card. The remaining screens
still show text glyphs (← ✓ ⧉), raw `user:5` targets in the audit log, bare tables and plain empty
texts. This step brings them to the same level and closes the minors left from steps 1–2.

## Goals

- No text glyphs used as icons anywhere in the web app.
- The audit log and the user card's History name people, houses and statuses, with links.
- Lists sit in cards; empty lists explain themselves and offer the next action.
- Users can be filtered by status in one click, with counts.
- The admin can pick light, dark or system theme.

## Non-goals

- New admin functions; changes to the public pages beyond the copy icon.

## Approaches

| Where names for the audit log come from | Trade-off |
|---|---|
| **The API adds names to each entry** (chosen) | One request; works on the audit page and in the card; a few extra lookups per page (≤ 25 entries) |
| The browser looks names up per entry | No API change, but many requests and flicker |
| Store names in the audit row when it is written | Fast reads, but names go stale after renames, and old rows have none |

## Design

- **API.**
  - Each audit entry gains `actorName` (string, empty when there is no actor or the user is gone),
    `targetName` (string: a user's full name, or their phone when nameless; a house's name; otherwise
    empty) and `targetNames` (a status's localized names, otherwise null).
  - `GET /admin/summary` adds `disabledUsers` and `rejectedUsers`.
- **Glyphs → icons.**
  - House page: breadcrumbs "Houses › name" and `PageHeader`.
  - Users list: a Telegram icon with a tooltip instead of ✓.
  - `QrLink`: copy/check icons.
- **Audit table.**
  - Target and actor show the name, linked: user → card, house → house page, status → statuses page.
  - Falls back to the raw target when there is no name.
- **Lists.**
  - Users, Houses, Statuses and Audit tables are wrapped in a `Card`.
  - New `EmptyState({ icon, title, hint?, action? })` used for empty applications, houses, users
    (no matches) and the audit log; the houses one offers "Add house".
- **Users status tabs.**
  - Tabs (pills) over the list: All · Waiting for approval · Waiting for the bot · Active · Disabled ·
    Rejected, each status with its count from the summary.
  - They drive the existing `?status=` filter and replace the status select.
- **Theme switch.** User menu items Light / Dark / System (Mantine colour scheme manager,
  remembered in the browser); the current one is checked.
- **Statuses.** The code column leaves the list; the code stays in the edit form.
- **Minors.**
  - Application rows and names wrap anywhere on phones.
  - The two copy buttons in the user summary are labelled "Copy phone" and "Copy link".
  - Unused keys `admin.applications.appliedAt` and `admin.user.access` are removed.
  - The Applications "Registration link" button stays enabled; the dialog shows the error if `/site`
    failed.
  - A failed QR download shows an error notification.

## Constraints

Every text in `web/src/locales/{uk,en}.json` with parity; synthetic test data; Mantine only.

## Left to the builder

Icons, spacing, wording details.
