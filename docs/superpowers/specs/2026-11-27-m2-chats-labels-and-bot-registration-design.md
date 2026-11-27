# M2: The bot in the community chats — chats, labels, notifications, registration from the bot — design

- **Date:** 2026-11-27
- **Status:** Approved by the owner (2026-11-27)
- **Owner:** Igor Artasevych
- **Tracks:** epic #3 (M2 Bot in the community chats)
- **Builds on:** [M0 + M1 design](2026-11-27-m0-m1-foundation-and-identity-design.md), [ADR-0001](../../adr/0001-stack-and-process-shape.md)

## 1. Intent

M1 gave the system its residents and admins. The bot, though, is blind to the groups it sits in: the
production bot is already in a group the owner plans as the **admins' journal** (later: gate
openings with camera snapshots, M6), the owner wrote there, and nothing shows up anywhere. The bot
subscribes only to `message` updates and hands group messages to a sink that drops them; it never
stores a chat.

M2 makes the system **know where it can write** and gives the owner a **flexible, readable way to
say what goes where**, so that later modules (M3+) and flows (M5) write to "the admins' journal",
not to a chat ID:

- The bot **remembers every group, forum and forum topic** it is in, from what Telegram tells it.
  No manual chat IDs.
- Admins **accept** chats (or make the bot leave them) and hang **labels** on them — a label is a
  named tag such as "journal" or "residents". Whoever sends a message sends it **to a label**; the
  system delivers it to every accepted chat or topic carrying that label. Moving the journal to
  another group means moving the label; nothing else changes.
- The first real sender is the core itself: **a new registration application is announced to the
  label the admin picks** (the admins' journal).
- **Registration can start in the bot**, not only from the link pinned in the community chat: a
  person shares their phone with the bot, gets the application form with the phone already filled
  in, and after approval is active at once and told so by the bot.

The owner's words on labels: a list of labels that connects different parts of the system, flexible
now and later ("потом мы разработаем модуль, который пишет в какую-то лейбу"), and understandable.

## 2. Goals, non-goals, anti-goals

### Goals

1. The bot receives `my_chat_member` updates and records every chat it is added to, promoted in,
   restricted in, or removed from. A chat it is already in without such an event (the existing
   journal group) is recorded the first time any message arrives from it.
2. Forum topics are discovered from messages (`message_thread_id`) and named from
   `forum_topic_created` / `forum_topic_edited`, including the topic-creation message Telegram
   attaches as `reply_to_message`. Unnamed topics can be named by an admin.
3. **Chat acceptance.** A chat the bot was added to by an active admin of the system is accepted
   at once. A chat it was added to by anyone else is left automatically. A chat discovered without
   an "added" event is listed as **new**, and an admin accepts it or makes the bot leave.
4. **Chat ID migration.** When a group becomes a supergroup (also when topics are turned on), the
   chat's new ID replaces the old one; its labels, topics and acceptance follow.
5. An admin **Chats** section: every chat and topic, its type, state, the bot's rights, its labels;
   actions: accept, leave, name a topic, edit labels, send a test message.
6. **Labels**: an admin-managed list (create, rename, describe, delete); a label can hang on any
   number of chats/topics, a chat/topic can carry any number of labels.
7. **Send to a label** through a durable **outbox** in the database, with retries, used by the
   core in M2 and by modules later.
8. **New application notice**: each new (or renewed) application is posted to the label chosen in
   a new **Notifications** setting.
9. **Registration from the bot** (2A): an unknown or rejected phone shared in the bot gets a
   one-time link to `/register` with the phone filled in and locked; the Telegram account is linked
   to the application; approval makes the person **active immediately**; the bot tells the person
   when the application is approved or rejected.
10. A person whose application from the web is still pending and who shares their phone in the
    bot gets their Telegram linked right away, so they too become active on approval and are
    notified.
11. **Bot setup guide** in `docs/deploy-railway.md`: BotFather privacy mode, `/setjoingroups`,
    admin rights, re-adding after a privacy change, how to check the bot sees a group.
12. All new texts in the bot and on the web go through i18n (uk default, en), with the existing
    parity tests.

### Non-goals (later)

- **Marking unidentified chat members** — postponed by the owner (2026-11-27). The bot can never
  list a group's members; it only sees who writes, who joins, and (as an admin) `chat_member`
  events. Revisit with that limit in mind.
- **Reacting to group messages.** Still not hard-coded: flows (M5) running WASM modules (M3). M2
  forwards messages from accepted chats to the existing `GroupSink`, which stays a `NopSink`.
- Storing group messages or members.
- Labels on anything other than chats and topics (the binding table is generic so this is cheap
  later).
- Modules or flows choosing a label (M3/M5); a bus (M3).
- Photos/attachments in outbox messages (M6 gate snapshots will extend the payload).
- A Telegram Mini App version of the form (2B) — possible later on top of 2A.
- Other messengers. The design keeps chats and labels messenger-neutral.
- The admin navigation redesign — a separate session works on it (see 5.8).

### Anti-goals (would be a failure even if it works)

- Anyone outside the admins able to make the system write into their group (adding the bot to a
  group must not make it a destination).
- A label that only works for Telegram, or a module that has to know a chat ID or a messenger.
- A lost or doubled application because a Telegram call failed; an application that fails because
  the journal is unreachable.
- A link from the bot that lets someone apply with a phone they did not prove.
- A phone number posted into a group (the system does not control who is in a group).
- Hard-coded strings outside locale files; real community names, chats or phones in the repo.

## 3. Constraints

Everything in M0/M1 section 3 still holds. New facts from Telegram that shape M2:

- **No list of chats.** The Bot API cannot list the chats a bot is in; it learns of a chat only
  from an update. A chat the bot joined before it subscribed to `my_chat_member` appears only when
  something happens in it.
- **No list of topics** and no list of members. Topic names arrive only with topic service
  messages (or the creation message attached as `reply_to_message` to topic messages).
- **Privacy mode** (BotFather) decides whether the bot sees all group messages. Turning it off
  applies only to groups the bot joins **after** the change; an administrator bot sees all messages
  regardless.
- `allowed_updates` is set per `getUpdates` call and per `setWebhook`; the bot must ask for
  `my_chat_member` explicitly (it is in the defaults, but the bot sends an explicit list today).
- A group turned into a supergroup gets a new ID: the old chat receives a message with
  `migrate_to_chat_id`, the new one a message with `migrate_from_chat_id`; sending to the old ID
  fails with an error carrying `migrate_to_chat_id`. Turning a supergroup into a forum keeps its ID.
- URL buttons must point to a public `https` URL (not `localhost`).
- Messages in the forum's General topic carry no `message_thread_id`; sending without one posts to
  General.

## 4. Approaches considered (picked by the owner, 2026-11-27)

### 1. Where chats and labels live

**1A. In the messenger-neutral core — chosen.** Chats, topics and labels are domain entities that
carry a messenger type and that messenger's IDs. The Telegram adapter only translates its updates
into neutral calls ("chat seen", "bot added/removed", "chat moved to a new ID", "topic seen/renamed").
Sending is "send to label": the core finds the accepted destinations and sends through the right
messenger. *Trade-off:* more code in the core; in return Discord later is a new adapter, while
labels, admin screens and modules stay as they are.

**1B. Event journal.** The adapter appends every chat event to a log and the chat list is a
projection of it. Prepares for flows, but the bus is planned for M3 anyway; extra moving parts now.

*Dropped:* Telegram-specific chat tables inside the Telegram package — fastest, but labels would be
tied to Telegram, against the messenger-agnostic rule.

### 2. Registration from the bot

**2A. The bot hands out a signed one-time link to the existing `/register` form — chosen.** One form,
the existing validation, rate limits and i18n.

**2B. Telegram Mini App** opening the same form inside Telegram. Nicer, but Telegram Desktop/Web
embed it in a frame, which the site's `frame-ancestors 'none'` forbids; would need loosening.
Possible later on top of 2A.

**2C. A dialogue in the bot** (name? house? apartment?). A second copy of the form, its texts and
checks to maintain forever.

### 3. How notifications are sent

**3A. An outbox table — chosen.** A notification is written in the same transaction as the event
(e.g. the application); a background worker sends it and retries. Nothing is lost on a Telegram
outage or a restart, and an application never fails because of sending. Modules will send through
it later.

**3B. Send right after commit.** Simpler; a failed send is lost (only a log line).

## 5. Design

### 5.1 Messenger boundary

The domain gains neutral types and the `Messenger` interface grows:

```go
// Destination is a place a messenger can post to: a chat, or a topic inside it.
type Destination struct {
    ChatID   string // messenger's chat ID
    ThreadID string // forum topic; "" = the chat itself (General in a forum)
}

type LinkButton struct{ Text, URL string }

type OutMessage struct {
    Text    string       // plain text, already localized
    Buttons []LinkButton // optional URL buttons
}

type Messenger interface {
    Type() string
    SendText(ctx, chatID, text string) error          // kept for login codes
    Send(ctx, to Destination, m OutMessage) error      // returns a *SendError on failure
    LeaveChat(ctx, chatID string) error
    BotURL() string
}
```

`SendError` classifies a failure so the outbox can act without knowing Telegram:
`Retry(after)` (rate limit, 5xx, network), `ChatGone` (bot removed / forbidden),
`ThreadGone` (topic deleted), `ChatMoved(newChatID)`, `Permanent` (any other 4xx).

The core keeps a small registry `map[type]Messenger` (only Telegram today).

**Inbound** (Telegram adapter → domain), a new `ChatEvents` interface the adapter calls:

| Telegram input | Domain call |
|---|---|
| `my_chat_member`, bot became member / administrator / restricted-but-member from left/kicked | `BotAdded(chat, addedBy externalID, botRights)` |
| `my_chat_member`, rights changed while present | `BotRightsChanged(chat, botRights)` |
| `my_chat_member`, bot left / kicked | `BotRemoved(chat)` |
| any group/supergroup message | `ChatSeen(chat)` (title, kind, is_forum) and, if `is_topic_message`, `TopicSeen(chat, threadID, name-if-known)` |
| `forum_topic_created` / `forum_topic_edited` | `TopicSeen(chat, threadID, name)` |
| `migrate_to_chat_id` in the old chat / `migrate_from_chat_id` in the new one | `ChatMoved(oldID, newID)` |

`chat` here is a neutral `ChatInfo{Type, ExternalID, Kind, Title, IsForum}`; `botRights` is
`{Status: member|administrator|restricted, CanSend bool}`. Private-chat `my_chat_member` updates
(a person blocking the bot) are ignored in M2.

`allowedUpdates` becomes `["message", "my_chat_member"]` for both polling and `setWebhook`.
When a chat is first recorded from a message (no `my_chat_member` seen), the adapter asks Telegram
for the bot's own membership (`getChatMember(chat, botID)`) to fill rights; a failure leaves them
unknown.

After recording, messages from **accepted** chats still go to `GroupSink` (a `NopSink`); others are
dropped.

### 5.2 Data model (new migration `0003_chats_labels_outbox.sql`)

| Table | Key fields | Notes |
|---|---|---|
| `chats` | id, messenger (`telegram`), external_id, kind (`group` / `supergroup` / `channel`), is_forum, title, state (`new` / `accepted` / `left`), bot_status (`member` / `administrator` / `restricted` / `unknown`), can_send, added_by_external_id, added_by_user_id (nullable), accepted_by, accepted_at, leave_at (nullable), first_seen_at, last_seen_at, left_at | Unique (messenger, external_id). Rows are never deleted by the bot, so a re-added chat keeps its labels. |
| `chat_topics` | id, chat_id → chats (cascade), external_id (thread id), title (`''` = unknown), title_set_by_admin, gone, first_seen_at, last_seen_at | Unique (chat_id, external_id). The General topic is the chat itself, not a row. |
| `labels` | id, name, description, created_at, updated_at | Name unique case-insensitively (application-side folding as for user search), 1–40 chars, free text in any language. |
| `label_bindings` | label_id → labels (cascade), target_type (`chat` / `chat_topic`), target_id, created_at | PK (label_id, target_type, target_id). Generic target so other parts of the system can carry labels later; deleting a chat/topic row deletes its bindings in the same transaction (code, not FK). |
| `outbox` | id, messenger, chat_id, thread_id, payload (JSON: text, buttons), origin (e.g. `application:42`, `test`), label_id (nullable), user_id (nullable), state (`pending` / `sent` / `failed`), attempts, next_attempt_at, last_error, created_at, sent_at | One row per destination. Kept 30 days after sent/failed, then deleted by the cleanup loop. |
| `registration_tokens` | id, token_hash, channel_type, external_id, chat_id, username, phone (E.164), expires_at, used_at, created_at | Bot-issued application links (5.5). |
| `settings` key `notifications` | `{"applicationsLabelId": 7}` | New key in the existing table. |

`chats.kind = channel` is accepted for Telegram channels the bot is made admin of: cheap to support
as a destination (Claude's call); channel posts are not read.

### 5.3 Chat lifecycle

```
            BotAdded by an active admin ─────────────────────┐
                                                             ▼
(unknown) ──ChatSeen (message, no add event)──▶ new ──accept──▶ accepted
    │                                            │                 │
    └──BotAdded by anyone else──▶ bot leaves ──▶ left ◀──leave─────┘
                                                 ▲   ◀──BotRemoved─┘
                                                 └── new: "leave"
left ──BotAdded by an active admin──▶ accepted (labels kept)
left ──BotAdded by anyone else──▶ bot leaves again, stays left
```

- "Active admin" = the `from` of the `my_chat_member` update is the linked Telegram account of a user
  with `status = active` and `is_admin = true`.
- **Grace before leaving** (Claude's call). An add by anyone else does not make the bot leave at
  once: the chat is recorded as `new` with `leave_at = now + 2 min`, and the cleanup loop leaves it
  when `leave_at` passes. Reason: when a group is upgraded to a supergroup (also when topics are
  turned on), Telegram may report the bot "added" to the new ID by whoever upgraded it — a group
  moderator who is not a system admin. If a `ChatMoved` merges the row with a known chat, or an
  admin accepts it, `leave_at` is cleared. A `new` chat receives nothing meanwhile.
- **Anonymous admins** (Claude's call). An add whose `from` is Telegram's anonymous-group-admin
  account (`GroupAnonymousBot`) cannot be attributed, so the chat is recorded as `new` without
  `leave_at`; an admin accepts or leaves it.
- "Leave" (admin action, or automatic) calls `LeaveChat`, then sets `left`; if Telegram says the bot
  is not there any more, the state is set anyway.
- Only `accepted` chats (and their non-gone topics) receive messages. Labels may be edited on `new`
  and `left` chats too (a left chat shows a warning that it receives nothing).
- `ChatMoved(old, new)`: if only the old row exists, its `external_id` becomes the new ID and `kind`
  becomes `supergroup`. If a row for the new ID already exists (seen first), bindings and topics of
  the old row move to it, it becomes `accepted` if either was, and the old row is deleted. Pending
  outbox rows for the old ID are re-pointed. Idempotent: the second of the two service messages
  finds nothing to do.
- Every state change is audited: `chat_added`, `chat_auto_left` (with who added it),
  `chat_accepted`, `chat_left`, `chat_removed`, `chat_moved`; label changes `label_created`,
  `label_updated`, `label_deleted`, `label_bound`, `label_unbound`; `notifications_updated`.

### 5.4 Labels, sending and the outbox

`Notifier.SendToLabel(ctx, labelID, msg OutMessage, origin)` (inside the caller's transaction):
resolves the label's bindings to accepted chats and non-gone topics of accepted chats, and inserts
one `outbox` row per destination (deduplicated). A label with no destination inserts nothing and
logs at info. `Notifier.SendToUser(ctx, userID, msg, origin)` inserts one row for the user's linked
channel (its private `chat_id`); nothing if the user has no channel.

**Worker** (one goroutine in the process, like the session cleanup): picks due `pending` rows
oldest first, sends one at a time (Telegram limits: at most ~20 messages/min into one group; the
worker spaces sends to the same chat by ≥ 3 s), and on the `SendError`:

- success → `sent`;
- `Retry(after)` → `next_attempt_at = now + max(after, backoff)`; backoff 10 s doubling to 30 min;
  after 24 h from `created_at` → `failed`;
- `ChatMoved(new)` → `ChatMoved` handling (5.3), then retry at once;
- `ChatGone` → the chat becomes `left` (audited), the row `failed`;
- `ThreadGone` → the topic is marked `gone`, the row `failed`;
- `Permanent` → `failed` with the error.

The worker wakes on a ticker (5 s) and immediately after a commit that enqueued rows (a non-blocking
signal). On start it resumes whatever is pending. One replica, so no locking beyond SQLite's.

Texts for a label are rendered in the site's default locale; texts for a user in the user's locale
(falling back as in M1). Rendering happens at enqueue time; the payload stores the final text.

**Test message.** On a chat, a topic or a label, "Send test message" enqueues a localized test
text with origin `test` and shows the result (sent / failed with reason) by polling the row.

### 5.5 Registration from the bot and notices to the person

**Bot, contact shared** (extends M1 activation; the "own contact" check stays):

| Phone / account state | Today (M1) | M2 |
|---|---|---|
| Unknown phone | "not registered, contact an admin" | **Issue a registration link**: "You are not registered yet — fill in the application, your number is already in it" + URL button |
| `rejected` | same neutral answer | **Issue a registration link** (rejected people may reapply) |
| `pending_approval`, no Telegram linked | "waiting for approval" | **Link this Telegram account to the application**, then "waiting for approval — I will write to you when it is decided" |
| `pending_approval`, this account already linked | — | "waiting for approval" |
| `awaiting_activation` | activate | unchanged |
| `active` with this account | "already connected" | unchanged |
| `disabled`; `active` with another account; this account linked to another phone | neutral "contact an admin" | unchanged |
| Bootstrap | unchanged | unchanged |

**Decision order** (Claude's call; replaces the early return in M1 `ActivateChannel`):

1. **The Telegram account is already linked** to a user U.
   - U has this phone: answer by U's status — `active` → already connected; `pending_approval` →
     waiting; `rejected` → issue a registration link (applying reuses U and keeps the link);
     anything else → neutral.
   - U has a different phone and U is `rejected` → treat the account as free (step 2); applying
     with the token moves the link from U to the applicant (audited `channel_unlinked` on U,
     `channel_linked` on the applicant).
   - U has a different phone and any other status → neutral.
2. **The account is free.** Look the phone up:
   - unknown → issue a registration link;
   - `rejected` with no Telegram linked, or linked to this account → issue a link;
     `rejected` with **another** Telegram account linked → neutral;
   - `pending_approval` with no Telegram linked → link this account, "waiting, I will write to you";
     with another account linked → neutral;
   - `awaiting_activation` → activate (M1);
   - `active`, `disabled` → neutral (an active user with another account was already neutral in M1).

On apply with a token, the same rules are checked again in the transaction (the state may have
changed within the hour); if linking is no longer allowed, the application is still taken (M1
path) without a link.

The M1 rule "one neutral answer for unknown / rejected / disabled" protected against probing other
people's numbers. In M2 the person has proven the number is theirs (shared contact, `user_id`
checked), so telling them the state of **their own** number leaks nothing. The answer for a phone
that belongs to someone else's linked account stays neutral.

**The link.** 32 random bytes, base64url; the database keeps an HMAC-SHA256 (with `APP_SECRET`) of
it in `registration_tokens` with the Telegram account, private chat id, username, verified phone,
`expires_at = now + 1 h`. A new link for the same Telegram account invalidates older unused ones.
The URL is `PUBLIC_URL/register#t=<token>`: the token sits in the fragment so it never reaches
server or proxy logs. If `PUBLIC_URL` is not `https`, the link goes into the text instead of a
button.

**The form.** `/register` reads `#t=`, calls `POST /api/v1/register/prefill {token}` → `{phone}`
or `invalid` (unknown, used, expired). With a valid token the phone field is filled and locked and
a note says it was confirmed in Telegram; with an invalid one the page says the link expired and
offers the bot button, and the ordinary form stays usable. Submitting sends the token along
(`POST /api/v1/register` gains an optional `token`).

**Apply with a token** (same transaction as M1 `Apply`): the token must be valid and unused; the
phone comes from the token (the form's phone is ignored); the token is marked used. Then as in M1
(new user, or a rejected row reused, or — if the phone meanwhile belongs to someone in another
state — the same silent success, token still used). For a created or reused user, the channel is
linked following the decision order above. The audit entry `user_applied` gets `"via": "bot"`.

**Approve.** A pending user **with** a Telegram channel becomes `active` directly (`activated_at =
now`, audit `user_approved` then `user_activated`), and `SendToUser` queues "Your application is
approved — log in here" with a button to `PUBLIC_URL/login`. Without a channel: M1 behaviour
(`awaiting_activation`, the person activates in the bot).

**Reject.** If the user has a channel, `SendToUser` queues a short neutral "Your application was
not approved; for questions contact the administrators". The channel stays linked, so the person
can reapply from the bot.

**Phone edits.** An admin may no longer edit the phone of a user who has a linked channel, whatever
the status (the phone is what Telegram proved); unlink first. M1 already had this rule for active
users.

### 5.6 New application notice

In `Apply`, when a user is created or a rejected one is renewed (not on the silent no-op), and the
`notifications.applicationsLabelId` setting points to a label, `SendToLabel` queues, in the default
locale:

```
📝 New application
Ivan Petrenko — house "A", apt 12, owner
Via: bot | site · Waiting: 3
[Open applications]   → PUBLIC_URL/admin/applications (or the current applications route)
```

No phone number. The setting lives in a new **Notifications** block in the admin (5.8) with one
row in M2: "New applications → label [select, empty = off]". Deleting a label that a setting uses
clears the setting (and the delete dialog says so).

### 5.7 HTTP surface (admin only unless noted)

- `GET /api/v1/admin/chats` — chats with their topics, labels, state, rights, last seen.
- `POST /api/v1/admin/chats/{id}/accept`, `POST .../leave`.
- `PUT /api/v1/admin/chats/{id}/labels` and `PUT /api/v1/admin/topics/{id}/labels` — replace the
  set (`[labelId]`).
- `PATCH /api/v1/admin/topics/{id}` — `{title}`.
- `POST /api/v1/admin/chats/{id}/test`, `POST /api/v1/admin/topics/{id}/test`,
  `POST /api/v1/admin/labels/{id}/test` → outbox row ids; `GET /api/v1/admin/outbox/{id}` → state.
- `GET/POST /api/v1/admin/labels`, `PATCH/DELETE /api/v1/admin/labels/{id}` (list includes where
  each label hangs and whether a setting uses it).
- `GET/PUT /api/v1/admin/settings/notifications`.
- Public: `POST /api/v1/register/prefill`; `POST /api/v1/register` accepts `token`.

Errors use the existing error envelope and codes; new codes: `token_invalid`, `label_name_taken`,
`chat_not_accepted` (test send to a chat that receives nothing).

### 5.8 Admin screens

A separate session is redesigning the admin navigation (owner's request, 2026-11-27). M2 adds two
places; where they sit in the menu follows that redesign if it lands first, otherwise a "Chats"
menu item with two tabs.

**Chats** — a tree table:

```
Chats                                                     [Labels]
┌──────────────────────────┬──────────────┬───────────────────┬──────────┬─────────────────┐
│ Name                     │ Type         │ Labels            │ Bot      │                 │
├──────────────────────────┼──────────────┼───────────────────┼──────────┼─────────────────┤
│ log            NEW       │ TG · group   │ + add             │ member   │ Accept · Leave  │
│ Our yard                 │ TG · forum   │ [residents] +     │ admin    │ Test · Leave    │
│   └ Announcements        │ TG · topic   │ [announcements] + │          │ Test · Rename   │
│   └ Topic #42 (unnamed)  │ TG · topic   │ +                 │          │ Test · Rename   │
│ Someone's group  LEFT    │ TG · group   │                   │ —        │ added by @x     │
└──────────────────────────┴──────────────┴───────────────────┴──────────┴─────────────────┘
```

- `new` chats first with a hint ("the bot is in this chat but nobody confirmed it"); `left` chats
  collapsed under "Left chats".
- A chat where the bot cannot send (`can_send = false`) shows a warning.
- An empty state explains how a chat appears (add the bot as an admin of the system; or write a
  message in a chat where the bot already is) and links to the setup guide.
- Labels are edited inline (multi-select with "create label …").

**Labels** — list: name, description, where it hangs (chips), "used by: New applications"; create,
rename, delete (confirmation lists what loses it), test send.

**Notifications** — the one row from 5.6, in the Site settings area or wherever the redesign puts
settings.

Badge: the admin menu shows a counter of `new` chats like the applications counter.

### 5.9 Bot texts (i18n keys, uk + en)

Registration link offer, pending-and-now-linked, approved notice, rejected notice, new application
notice (group), test message, and the private-chat help updated to mention registration. Group
messages are never answered in M2.

### 5.10 Bot setup guide (docs)

A section in `docs/deploy-railway.md` (or a new `docs/telegram-bot-setup.md` linked from it):

1. BotFather: privacy mode **off** (`/setprivacy` → Disable); note that it applies to groups joined
   after the change — re-add the bot to groups it was already in, or make it an administrator.
2. BotFather `/setjoingroups`: enable while adding the bot to the system's groups, then disable, so
   nobody else can add it (the system also leaves groups added by non-admins).
3. Rights: in the journal a regular member is enough; in the community group make the bot an
   administrator with no extra rights (sees all messages; needed later for member events).
4. Check: add the bot (as a system admin) to a test group → it appears in Chats as accepted; write a
   message in a group the bot was already in → it appears as new.
5. For the existing journal group: after deploying M2, remove the bot from it and add it again.

### 5.11 Testing

- Domain: chat lifecycle (every transition in 5.3, including added-by-non-admin → leave after the grace, grace cancelled by a
  migration or an accept, anonymous-admin add → new,
  re-add keeps labels, `ChatMoved` in both orders and twice), topics (named by created/edited, by
  `reply_to_message`, unnamed, admin rename then a Telegram rename), label CRUD (name uniqueness
  with case folding, delete clears the setting), `SendToLabel` resolution (only accepted, not gone,
  dedup, empty label), outbox worker on each `SendError` kind with a fake messenger and fake clock,
  per-chat spacing, restart resumes pending rows.
- Registration: each row of the 5.5 table and each branch of the decision order; token expiry, reuse, older token invalidated, phone in
  the form ignored, token for a phone that became active meanwhile; approve with/without channel
  (active vs awaiting_activation; notices queued in the right locale); reject notice; phone edit
  refused with a channel; application notice queued only for created/renewed applications and
  contains no phone.
- Telegram adapter: update → domain-call mapping for each row of 5.1 (recorded fixtures of real
  update JSON with synthetic ids), `allowed_updates` in polling and webhook, error classification
  of Bot API errors into `SendError`, URL buttons vs text links for non-https `PUBLIC_URL`.
- HTTP: admin endpoints (admin only), prefill/apply with token.
- Web: Chats table states, label editing, notifications setting, register page with a valid /
  invalid token; locale parity.
- Migration test for `0003`.

## 6. Left to the builder

Package and type names; whether chat/label services are one or two domain services; the exact
outbox polling/backoff numbers within the spirit of 5.4; how fixtures of Telegram updates are
stored; table layout details on the Chats page; emoji in notices; whether `getChatMember` on first
sight runs inline or in the background; route names on the web.

## 7. Calls made by Claude (for the owner to check)

- Labels are single names (no "tg:" prefix); the destination shows its own type. One label may
  hang on many chats (a message goes to all of them).
- The core's own notices are routed by a setting that picks a label, the same way modules will.
- Chats added by anyone who is not an active admin of the system are left automatically after a
  2-minute grace (so a group upgrade does not throw the bot out) and kept in the list as "left"
  with who added them; adds by an anonymous group admin and chats found without an add event wait
  as "new" for an admin.
- When a Telegram account is already linked to another user, the order in 5.5 decides; a link held
  by a rejected user is moved to the new applicant, any other link gives the neutral answer.
- Telegram channels are accepted as destinations (send-only).
- A topic title from Telegram overwrites an admin's name for it.
- Outbox rows are rendered at enqueue time; label messages use the site's default language.
- Sending to one chat is spaced ≥ 3 s; retries stop after 24 h.
- The bot now tells a person the state of their own proven number (unknown / rejected / pending);
  numbers linked to someone else still get the neutral answer.
- A web applicant who later shares their phone in the bot gets linked while pending, and is then
  activated directly on approval.
- Approved and rejected people with a linked Telegram are notified; the rejection text is neutral.
- The bot's registration link lives 1 hour, is single-use, and travels in the URL fragment.
- No phone numbers in group notices.
- An admin cannot edit the phone of any user with a linked channel.
