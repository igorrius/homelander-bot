# Deploying on Railway

One Homelander deployment serves one community: one Railway project with one service and one
volume. The repository is public, so nothing about a particular deployment goes into it:

- the infrastructure lives in `.railway/railway.ts`, which is **git-ignored** — you create it from
  the template [`docs/railway.example.ts`](railway.example.ts) and keep it on your computer;
- secrets and personal data are typed into the Railway UI only.

## What you need

- A Railway account and the CLI (`railway login`).
- Node.js, then `npm install` in the repository root (the Railway SDK that reads `.railway/railway.ts`).
- `openssl` for generating secrets.
- A phone with Telegram — the number that becomes the first admin.

## 1. Create the production bot

1. In @BotFather: `/newbot`, pick a name and a username. Keep the token for step 4. Use a different
   bot from the one you develop with.
2. `/setprivacy` → choose the bot → **Disable**. Without this the bot sees only commands and
   replies in groups, not the messages it will later react to.
3. Optional: `/setdescription` and `/setuserpic` — residents see them before pressing Start.

## 2. Create and link the Railway project

1. In the Railway UI create an **empty project**. Name it as you like; the name stays in Railway.
2. In the repository root:

   ```bash
   railway link
   ```

   and pick that project.

## 3. Apply the infrastructure

```bash
mkdir -p .railway && cp docs/railway.example.ts .railway/railway.ts
```

In `.railway/railway.ts` set `projectName` to the project name exactly as in Railway (leave
`domain` empty for now). Then:

```bash
railway config plan
```

Read the plan: it should create the service `homelander` (from GitHub `igorrius/homelander-bot`,
branch `main`, built from the `Dockerfile`), the volume `homelander-data` mounted at `/data`, and
the variables listed in `.railway/railway.ts`. Nothing should be deleted. Then:

```bash
railway config apply
```

The first deployment fails: the secrets are not set yet. That is expected.

## 4. Set the secrets in the Railway UI

Service `homelander` → **Variables**. Add:

| Variable | Value |
|---|---|
| `APP_SECRET` | output of `openssl rand -hex 32` |
| `TELEGRAM_WEBHOOK_SECRET` | output of another `openssl rand -hex 32` (only `A-Z a-z 0-9 _ -` are allowed) |
| `TELEGRAM_BOT_TOKEN` | the token from step 1 |
| `BOOTSTRAP_ADMIN_PHONE` | your phone, e.g. `+380…`; used only until the first admin exists |

Never paste these values into a chat, an issue or a file in the repository. `.railway/railway.ts`
keeps them with `preserve()`, so later `apply` runs leave them as they are.

## 5. Give the service a domain

Service → **Settings → Networking → Generate Domain** (port `8080`). Then write it into `domain` in
`.railway/railway.ts` so later plans keep it:

```bash
railway config plan   # should show no changes, or only the domain
```

`PUBLIC_URL` follows the domain by itself (`https://${{RAILWAY_PUBLIC_DOMAIN}}`). Redeploy the
service so it starts with the secrets and the domain.

## 6. Turn on volume backups

Volume `homelander-data` → **Backups** → enable a **daily** schedule (and weekly, if you like). This
is the minimum before residents' phone numbers are in the database (issue #26 tracks a fuller
backup plan).

## 7. Check that it runs

- `https://<domain>/healthz` answers `{"status":"ok"}`.
- The deploy logs show `listening` and no `telegram setWebhook failed`.
- Telegram sees the webhook — run this yourself, it contains the token:

  ```bash
  curl -s "https://api.telegram.org/bot<TOKEN>/getWebhookInfo"
  ```

  `url` must be `https://<domain>/telegram/webhook/…` and `last_error_message` empty.

## 8. Become the first admin

1. Open the bot, press **Start**, then **Share my phone number** (from the phone in
   `BOOTSTRAP_ADMIN_PHONE`). The bot answers with a link to the admin panel.
2. Open `https://<domain>/login`, enter the phone; the code arrives in the bot.
3. In the admin panel, open your user card and fill in your name.
4. **Audit log**: the IP of your login should be your real address, not a `10.x` / `100.64.x`
   one. If it is internal, `TRUST_PROXY` does not see the client and all rate limits would be
   shared by everyone — report it before inviting residents.

## 9. Configure the community

1. **Site**: community name and welcome text (Ukrainian and English), optionally a picture of the
   houses (PNG, JPEG or WebP up to 5 MB). Without a picture the page draws the houses itself.
2. **Houses**: one per building or section — name, address, floors, the apartment number range.
3. **Residency statuses**: owner and tenant exist; add others if you need them.
4. Add the bot to the community group or forum. It reads messages but does not answer there.
5. Pin the registration link from **Site** (`https://<domain>/register`) in the chat. Review the
   applications as they come in; approved residents activate themselves in the bot.

## Updating

Every merge into `main` deploys automatically once the GitHub checks pass. Database migrations run
at start-up and only move forward.

Before changing `.railway/railway.ts`, always run `railway config plan`. Never remove the volume
from the file: Railway would delete it and the data with it. When the template in
`docs/railway.example.ts` changes, carry the change into your local copy by hand.

## When something is wrong

| Symptom | Cause and fix |
|---|---|
| Log: `configuration: … is required` | A secret from step 4 is missing. |
| Log: `unable to open database file` | `RAILWAY_RUN_UID=0` is missing; the volume belongs to root. |
| `502` from Railway | The app listens on another port: do not set `HTTP_ADDR`, Railway passes `PORT`. |
| The bot is silent | `getWebhookInfo` shows the error. `401` means `TELEGRAM_WEBHOOK_SECRET` changed after the webhook was set — redeploy. |
| The bot does not see group messages | Privacy mode is still on (step 1.2), or the bot was added before it was turned off — remove and add it again. |
| Login code never arrives | The user has not activated the bot, or blocked it; the log shows `login code not delivered`. |
