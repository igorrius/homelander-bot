# Labels on users — design

Short design. Builds on the [M2 design](2026-11-27-m2-chats-labels-and-bot-registration-design.md)
(labels, `label_bindings`, the outbox) and the
[cabinet and feed design](2026-11-28-resident-cabinet-and-feed-design.md) (feed publish to labels).

## Intent

The owner wants to hang labels on people, not only on chats and topics. A message sent to a label
then also reaches every person carrying it, in a private message from the bot. Two uses:

- **Testing without disturbing anyone.** Hang "testers" on yourself (or a few people) and send a
  feed post or a test message to it: no separate test chat is needed, and nobody else gets it.
- **Interest groups.** E.g. "house engineering community": notices only its members receive.

## Goals

- A label can hang on any number of users; a user can carry any number of labels.
- Sending to a label (feed publish, new-application notice, test message) delivers to the label's
  accepted chats/topics **and** to its users, via the same outbox.
- Admins edit a user's labels on the user card; the Labels page shows on whom each label hangs.

## Non-goals

- **Residents choosing labels themselves** (self-subscription in the cabinet). Owner's decision:
  only admins put labels on users.
- **Data visibility by label** ("only engineers see this"). Labels on users are for delivery only.
- Labels on anything else (houses, apartments, modules).

## Anti-goals

- A message reaching someone who should not get it: a disabled, rejected or not-yet-active user.
- A second "groups" concept next to labels — modules and the feed keep choosing from one list.

## Approaches

- **Users as one more label target — chosen.** `label_bindings` was made generic for exactly this;
  add the target type `user`. Senders don't change: they already send to a label.
- *Dropped:* a separate "groups" entity — duplicates labels; the feed and modules would have to
  pick both a label and a group, against the owner's "one list of labels connects the system".
- *Dropped:* filters on user flags ("all residents") — cannot express testers or arbitrary groups.

## Design

- **Migration `0010_user_labels.sql`:** rebuild `label_bindings` so the `target_type` CHECK allows
  `'chat', 'chat_topic', 'user'` (SQLite: create new table, copy, drop, rename, recreate the index).
- **Domain:** `TargetUser` next to `TargetChat`/`TargetTopic`. `Notifier.target` resolves a user
  target to the user's linked messenger channel (the same lookup `SendToUser` uses) and records
  `user_id` on the outbox row. A user target receives nothing unless the user is `active` and has a
  linked channel. Users are never deleted today; if deletion is added, it must `DeleteTarget` the
  user's bindings in the same transaction.
- **Deduplication** stays per destination as now: a user who carries two chosen labels gets one
  message. A user who is also in a chat carrying the label gets both (the private message and the
  chat post) — we cannot know they read that chat.
- **Text locale:** messages to a label keep the site's default locale (as M2), also for users —
  one rendering per send. *(Builder's call if it is cheap to render per user locale instead.)*
- **API:** `PUT /api/v1/admin/users/{id}/labels` replaces the set (`[labelId]`), like chats and
  topics. The user card response includes `labelIds`. The label list's `targets` may now contain
  `{type: "user", id}`.
- **Audit:** the existing `label_bound` / `label_unbound` actions, with the user as target.
- **Admin UI:**
  - User card: a labels multi-select (same control as on Chats, with "create label …").
  - Labels page: "where it hangs" shows user chips (name) next to chat/topic chips.
  - Feed: the label picker and default-labels setting offer labels that reach an accepted chat
    **or** an active user with a linked channel (`reachesChat` → a "reaches someone" check).
  - "Send test message" on a label also goes to its users.
- **Tests:** binding a user, resolution (active + linked only; disabled/rejected/awaiting/unlinked
  get nothing), dedup across two labels, chat + user both receive, feed publish to a user-only
  label, migration keeps existing bindings, web: user card label editing, feed picker offers a
  user-only label.

## Left to the builder

Naming, whether user resolution lives in `Notifier.target` or a sibling helper, and the exact
chip layout on the Labels page.
