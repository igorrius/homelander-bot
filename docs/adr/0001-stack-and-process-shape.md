# ADR-0001: One Go binary with SQLite, a React web app, and identity that starts on the web

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Igor Artasevych
- **Spec:** [M0 + M1 design](../superpowers/specs/2026-11-27-m0-m1-foundation-and-identity-design.md), sections 3 and 4

## Context

Homelander serves one resident community: a few hundred people, one or several houses, shared
chats. The first deployment runs on Railway (one service, one replica, a volume); the same build must
also run at home behind NAT. The repository is public. Later milestones add WASM modules and a
visual flow editor.

## Options

- **Process shape:** one binary (API, web app, bot worker) or separate services.
- **Storage:** SQLite through the pure-Go `modernc.org/sqlite` driver, or a server database.
- **Web framework:** React or Svelte. The planned flow editor uses xyflow, which exists for both.
- **Where identity starts:** in the bot (Telegram first) or on the web with the messenger linked as
  a channel.

## Decision

- **One Go binary**, internally split into packages. Separate services would only add cost at this
  size and on one replica.
- **SQLite, pure Go, behind repository interfaces**, so another database can be added without
  touching the domain. No CGO; one static binary.
- **React + TypeScript** with Vite and Mantine. React Flow is the more mature xyflow flavour, and
  React has far more ready-made components for tables and forms.
- **Web-first identity** (chosen by the owner): people apply through one public registration link,
  an admin approves, and every account is activated in the bot by sharing the person's own phone
  number. Telegram is one linked channel, so another messenger is a new channel type, not a second
  sign-up.
- **Configuration only from environment variables** and settings in the database; nothing about a
  community lives in the repository.

## Consequences

- One replica only: in-memory rate limits and a single SQLite writer are acceptable.
- The web build is embedded with `go:embed`; `web/dist/.gitkeep` is committed so Go compiles without
  a web build.
- Database backups need their own solution (issue #26).
