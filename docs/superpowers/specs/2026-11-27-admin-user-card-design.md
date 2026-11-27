# Admin user card in two columns with tabs (step 2) — design

- **Date:** 2026-11-27
- **Status:** Approved by the owner (2026-11-27)
- **Issue:** #35 · **Builds on:** [step 1](2026-11-27-admin-navigation-and-overview-design.md)

## Intent

The user card (`/admin/users/:id`) is one long column of six blocks (header, details, access,
messengers, apartments, history) that grows down the page. The owner wants it to stop sprawling and
to look like the rest of the reworked admin.

## Goals

- The person's summary and the main actions are always visible without scrolling.
- Long or rarely used sections sit in tabs.
- An admin can send a person who has not activated yet the bot link right from the card.

## Non-goals

- No API changes and no new user functions; every existing action stays.
- Polishing other admin tables, names in the audit log (later steps).

## Approaches

| Approach | Trade-off |
|---|---|
| **Summary column + tabs** (chosen) | Summary and actions always visible; sections one click away |
| Keep one column, collapse sections into accordions | Less change, but the page still grows and actions scroll away |
| Split into sub-pages per section | Clean URLs, but more clicks and more routing for little gain |

## Design

- **Breadcrumbs** "Users › Name" replace the "← Users" link; the page uses the step 1 visual base.
- **Left column (summary)**, a card: avatar with initials, full name, status badge, source badge;
  phone with a copy button; Telegram `@username` (or "not linked"); Resident / Admin switches; dates
  (created, approved, activated); main actions by status (Approve + Reject; Disable; Enable). One
  error alert for the switches and actions, as today.
- **Bot link block** in the summary, only for `awaiting_activation`: a short hint, the bot link
  (`site.botUrl`) with copy, and its QR code.
- **Right column, tabs:** Details (form) · Apartments (count) · Messengers (count) · History. The
  open tab is in the URL (`?tab=details|apartments|channels|history`, default `details`).
- **Phone:** summary first, tabs below. Desktop: summary 1/3, tabs 2/3.
- Apartment edit/remove use icons (Tabler) instead of the ✎ / ✕ glyphs.

## Constraints

Every text in `web/src/locales/{uk,en}.json` with parity; synthetic test data; Mantine only.

## Left to the builder

Exact widths, spacing and icons; component split inside the page.
