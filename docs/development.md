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
The Vite dev server proxies `/api` and `/healthz` to `localhost:8080`. The session cookie
(`__Host-session`, `Secure`) works on `http://localhost` in current browsers; use `localhost`, not
`127.0.0.1` or a LAN address.

## First run

1. Set `BOOTSTRAP_ADMIN_PHONE`, open the bot, send `/start` and share your contact: you become the
   first admin.
2. Log in at `/login` (the code arrives in Telegram) and fill in your name on your user card.
3. Add the houses (Admin → Houses) and, if needed, residency statuses; registration opens once at
   least one house and one status exist.
4. Set the community name, welcome text and picture (Admin → Site). The admin home page (Admin →
   Overview) lists what is still missing and shows the registration link with its QR code: pin it
   in the community chat.

## Web app

- `web/src/api/` — DTO types (`types.ts`) and one TanStack Query hook per endpoint (`public.ts`,
  `admin.ts`). All calls go through `api()` in `client.ts`.
- `web/src/pages/` — one file per screen; `web/src/pages/admin/` for the admin panel.
- The API answers with error codes only. `lib/errors.ts` turns them into texts:
  `errors.<code>` for the request and `validation.<reason>` for each field.
- Admin-entered texts are per locale; `lib/localized.ts` picks the current language, then the
  site's default language, then any filled value.
- Tests render the whole app with `renderApp(path)` (`src/test/render.tsx`) over a fake API from
  `mockApi(routes)` (`src/test/api.ts`), with synthetic fixtures from `src/test/fixtures.ts`.

## The Telegram bot

- Create a bot with @BotFather and put its token in `TELEGRAM_BOT_TOKEN` (use a separate test bot locally).
- The bot reads the community chats it is added to. To receive every group message, turn
  **privacy mode off** in @BotFather (`/setprivacy` → Disable) or make the bot an admin of the group;
  with privacy mode on, Telegram delivers only commands and replies to the bot. In M1 the bot never
  answers in groups; reactions come later as flows.
- Locally the bot uses long polling (`TELEGRAM_UPDATES=polling`); on Railway use `webhook`. The
  webhook path is derived from `APP_SECRET`, and Telegram's calls must carry `TELEGRAM_WEBHOOK_SECRET`.

## Deploying on Railway

See [deploy-railway.md](deploy-railway.md): the infrastructure is a local, git-ignored
`.railway/railway.ts` made from `docs/railway.example.ts`; secrets are set in the Railway UI.

## Conventions

- Documentation, code comments and commit messages are in English.
- No real addresses, phone numbers or community names anywhere in the repository, tests included.
  Use synthetic values such as `+380000000000`.
- Every user-facing text goes through the locale files: `locales/*.json` for the backend and bot,
  `web/src/locales/*.json` for the web app. Tests fail when the languages have different keys.
- Database changes are new files in `internal/storage/sqlite/migrations/` named `NNNN_name.sql`;
  migrations only move forward.
- `main` is protected: work on a branch and open a pull request (squash merge).
