# Admin panel: navigation, overview and visual polish (step 1) — design

- **Date:** 2026-11-27
- **Status:** Approved by the owner (2026-11-27)
- **Builds on:** [M0 + M1 design](2026-11-27-m0-m1-foundation-and-identity-design.md), section 5.6

## Intent

The owner "barely found" the public registration link: it sits at the bottom of Admin → Site, below
the texts and the picture. More broadly the admin panel looks bare (default Mantine, no icons, no
theme) and its menu is a flat list of six equal items that mixes daily work (applications, users)
with one-time setup (houses, statuses, site) and rare checks (audit). The owner wants the admin to
look like a professional product, step by step, in a light, flat style.

This is **step 1**: structure, navigation, a home page with the most-used actions, and a visual
base the later steps build on.

## Goals

- The registration link and its QR code are one click from anywhere in the admin, and the first
  thing an admin sees after logging in.
- The menu groups pages by how often they are used and leaves room for M2's "Chats".
- A visual base: an app theme, icons in the menu, buttons and page headers, consistent page headers.
- Approving an application works on a phone without scrolling sideways.
- A first-run admin sees what to set up before registration can open.

## Non-goals (later steps)

- User card redesign (two columns: summary + tabs). Step 2.
- Polishing the remaining tables and empty states, names instead of `user:1` in the audit log,
  a manual dark-mode switch.
- The M2 "Chats" section itself.
- An MCP server for managing the system through an AI assistant (separate issue).

## Anti-goals

- Changing the component library or adding Tailwind (ADR-0001 stays as is).
- Losing any existing admin function or breaking i18n parity.

## Approaches

| Approach | Trade-off |
|---|---|
| **Keep Mantine, add a theme, Tabler icons and an overview page** (chosen) | Same components and tests; the "bare" look came from missing theme/icons/layout, not from the library |
| Switch to Ant Design | Recognisable enterprise look, rich tables; every screen rewritten, heavier, more "banking" than light |
| Switch to shadcn/ui + Tailwind | Fashionable minimal look; every screen rewritten and Tailwind, which M1 deliberately avoided |

The owner chose to keep Mantine and improve the admin in steps.

## Design

**Theme.** `createTheme` in `Providers`: one primary colour, `md` radius, cards with a border by
default, system font stack. Works with the existing automatic light/dark scheme.

**Icons.** `@tabler/icons-react` (MIT). Used in the menu, page headers and main buttons.

**Menu** (left navbar, burger on phones):

- Overview (`/admin`)
- **People:** Applications (pending counter), Users
- **Community:** Houses — M2 adds Chats here
- **Settings:** Site, Residency statuses, Audit log

**Header.** App icon and "Admin panel" on the left; language switcher and a user menu (name →
resident page, log out) on the right.

**Overview page** (`/admin`, replaces the redirect to Applications; login already lands on `/admin`):

1. Setup checklist, shown only while registration cannot open: "add a house", "add a residency
   status" (and "fill in the community name" as an optional item), each linking to its page.
2. Counters: new applications, approved but not yet activated in the bot, active accounts, houses.
   Each links to the matching filtered list.
3. "Invite residents" card: QR code, the link, Copy, Download QR (PNG), a one-line hint to pin it
   in the community chat.
4. Latest applications (up to 5) with Approve / Reject and "All applications".

**Registration link.** Moves from Site to Overview. The Applications page gets a "Registration
link" button that opens the same QR/link block in a modal. Site keeps only how the public page
looks: texts, picture, default language.

**Applications list.** A list of rows that wrap on narrow screens (name and details, then the
buttons) instead of a table, so Approve / Reject stay visible on a phone. Shared by the
Applications page and the Overview.

**Page headers.** One `PageHeader` component: icon, title, optional description, actions on the
right. Used on every admin page.

**Backend.** `GET /api/v1/admin/summary` adds `awaitingActivation` and `activeUsers` next to
`pendingApplications` (all from the existing `CountByStatus`). Houses and statuses for the counters
and the checklist come from the existing list endpoints.

## Constraints

- Every text in `web/src/locales/{uk,en}.json`; key-parity test stays green. Ukrainian default.
- Synthetic data only in tests and fixtures.
- `main` is protected: one branch, one PR; the owner merges.

## Left to the builder

Exact colours, spacing and icon choices; component and file names; whether the Download QR button
renders a hidden canvas or converts the SVG.
