# Homelander

A bot and a web system for a residential community: the people of one or several houses who share
common chats. It knows who the residents are (verified phone number and messenger account), lets a
few admins manage houses, apartments and people, and will post automatic information into the
community chats through optional modules.

Status: early development. See the [design](docs/superpowers/specs/2026-11-27-m0-m1-foundation-and-identity-design.md)
and the [project board](https://github.com/users/igorrius/projects/15).

## Quick start

```bash
cp .env.example .env
make build
make run
```

Configuration comes only from environment variables; see [.env.example](.env.example).
Details: [docs/development.md](docs/development.md). Decisions: [docs/adr](docs/adr/README.md).

## Licence

[MIT](LICENSE)
