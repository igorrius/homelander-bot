# Development

## Prerequisites

- Go 1.27
- Node.js 26 and npm
- golangci-lint 2.15 (for `make lint`)
- Docker (optional, for the image)

## Setup

```bash
cp .env.example .env   # placeholders only; put a real test bot token here, never commit it
make build             # builds the web app and the binary
make run               # runs with the variables from .env
```

Open http://localhost:8080. The health check is `/healthz`.

## Day-to-day

| Command | What it does |
|---|---|
| `make test` | Go tests with the race detector, then web tests |
| `make lint` | `go vet`, golangci-lint, oxlint and the TypeScript check |
| `make web` | Builds the web app into `web/dist` |
| `make build` | Web app + binary with the web app embedded |
| `make docker` | Builds the container image |

For fast front-end work run the Go server (`make run`) and, in another terminal, `cd web && npm run dev`.
The Vite dev server proxies `/api` and `/healthz` to `localhost:8080`.

## Deploying on Railway

- One service built from the `Dockerfile`, one replica.
- A volume mounted at `/data` (the database lives at `/data/homelander.db`).
- Railway mounts volumes owned by root, while the image runs as the unprivileged `nonroot`
  user. Set `RAILWAY_RUN_UID=0` on the service so the process can write to the volume
  (Railway's documented setting for non-root images).
- Variables: `APP_SECRET`, `PUBLIC_URL=https://<service domain>`, `TELEGRAM_BOT_TOKEN`,
  `TELEGRAM_UPDATES=webhook`, `TELEGRAM_WEBHOOK_SECRET`, optionally `BOOTSTRAP_ADMIN_PHONE`.
  Do not set `HTTP_ADDR`: the server listens on Railway's `$PORT`.
- Health check path: `/healthz`.

## Conventions

- Documentation, code comments and commit messages are in English.
- No real addresses, phone numbers or community names anywhere in the repository, tests included.
  Use synthetic values such as `+380000000000`.
- Every user-facing text goes through the locale files: `locales/*.json` for the backend and bot,
  `web/src/locales/*.json` for the web app. Tests fail when the languages have different keys.
- Database changes are new files in `internal/storage/sqlite/migrations/` named `NNNN_name.sql`;
  migrations only move forward.
- `main` is protected: work on a branch and open a pull request (squash merge).
