# Labels on users Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admins can hang labels on users; anything sent to a label also reaches those users in a private bot message.

**Architecture:** Users become a third label target type (`user`) in the existing generic `label_bindings` table. `Notifier` resolves a user target to the user's linked messenger channel (active users only), so every existing sender (feed publish, application notice, label test) reaches users unchanged. The admin UI edits a user's labels on the user card, and label lists learn which users a label reaches.

**Tech Stack:** Go (domain, `internal/storage/sqlite`, `internal/httpapi`), SQLite migrations, React + Mantine + TanStack Query + i18next (web), vitest.

**Spec:** `docs/superpowers/specs/2026-11-28-user-labels-design.md`

## Global Constraints

- Only admins put labels on users (admin API only; nothing in the cabinet).
- A user target receives a message only when the user's status is `active` **and** they have a linked messenger channel.
- Dedup stays per destination: a user carrying two chosen labels gets one message; a user who is also in a chat carrying the label gets both.
- Label texts keep the site's default locale (unchanged `Notifier` behaviour).
- Label changes are audited with the existing `ActLabelBound` / `ActLabelUnbound`, target `domain.UserTarget(id)` (`"user:N"`).
- `SendToUser` keeps working for non-active users (application/approval notices) — do not add the status check there.
- Work on branch `feat/user-labels`; `main` is PR-only. Commit the spec and this plan with Task 1.
- Before each commit: `go test -race ./...`; web tasks also `cd web && npm test && npm run lint && npm run typecheck`.

## Review Focus

1. **A labelled user is disabled later** → stops receiving at once (resolution checks status at send time, not at bind time). Test in Task 2.
2. **A labelled user unlinks Telegram** → gets nothing, and the feed picker no longer counts the label as reaching someone. Tests in Tasks 2 and 3.
3. **A label hangs only on users** → `TestLabel` succeeds (does not return `ErrChatNotAccepted`), the feed picker and Notifications page offer it without the "goes nowhere" warning. Tests in Tasks 2 and 4.
4. **Existing chat/topic bindings survive the table rebuild.** Test in Task 1.
5. **Unknown user id in `PUT /admin/users/{id}/labels`** → 404, nothing bound. Tests in Tasks 2 and 3.

---

### Task 1: Allow `user` in `label_bindings`

**Files:**
- Create: `internal/storage/sqlite/migrations/0010_user_labels.sql`
- Test: `internal/storage/sqlite/migrate_test.go`, `internal/storage/sqlite/labels_test.go`
- Modify: `internal/domain/label.go` (add the constant only)

**Interfaces:**
- Produces: `domain.TargetUser = "user"`; table `label_bindings` accepting `target_type IN ('chat', 'chat_topic', 'user')`.

- [ ] **Step 1: Create branch and commit the docs**

```bash
git switch -c feat/user-labels
git add docs/superpowers/specs/2026-11-28-user-labels-design.md docs/superpowers/plans/2026-11-28-user-labels.md
git commit -m "docs: design and plan for labels on users"
```

- [ ] **Step 2: Write failing tests**

`TestMigrationUserLabelsKeepsBindings` in `migrate_test.go` (pattern of `TestMigrationDataVisibility`): migrate `upTo(t, 9)`, insert a label and bindings `('chat', 1)` and `('chat_topic', 2)`, migrate fully; assert both rows still exist with their `created_at`, an insert with `target_type = 'user'` succeeds, one with `'house'` fails, and the index `label_bindings_target` exists (`SELECT count(*) FROM sqlite_master WHERE type='index' AND name='label_bindings_target'` = 1).

`TestLabelBindingsOnUser` in `labels_test.go`: `SetLabelsOf(LabelTarget{TargetUser, 7}, [a, b])`, `LabelsOf` returns `[a, b]`, `Targets(a)` contains `{user, 7}`, deleting label `a` cascades its user binding.

- [ ] **Step 3: Run to see them fail**

Run: `go test ./internal/storage/sqlite/ -run 'TestMigrationUserLabels|TestLabelBindingsOnUser' -v`
Expected: FAIL (`TargetUser` undefined / CHECK constraint failed).

- [ ] **Step 4: Implement**

`0010_user_labels.sql`: header comment pointing at the spec; `CREATE TABLE label_bindings_new` identical to 0003's but with `CHECK (target_type IN ('chat', 'chat_topic', 'user'))`, `INSERT INTO label_bindings_new SELECT label_id, target_type, target_id, created_at FROM label_bindings`, `DROP TABLE label_bindings`, `ALTER TABLE label_bindings_new RENAME TO label_bindings`, recreate `CREATE INDEX label_bindings_target ON label_bindings(target_type, target_id)`. Nothing references `label_bindings`, so the drop is safe inside the migration transaction.

In `label.go` add `TargetUser = "user"` to the target-type constants.

- [ ] **Step 5: Run the tests**

Run: `go test -race ./internal/storage/sqlite/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/sqlite internal/domain/label.go
git commit -m "feat(storage): labels can hang on users"
```

---

### Task 2: Domain — bind labels to users and deliver to them

**Files:**
- Modify: `internal/domain/label_service.go`, `internal/domain/notifier.go`, `internal/domain/users_service.go`
- Test: `internal/domain/label_service_test.go`, `internal/domain/notifier_test.go`

**Interfaces:**
- Consumes: `domain.TargetUser` (Task 1).
- Produces:
  - `LabelService.SetLabels(ctx, LabelTarget{TargetUser, userID}, ids, actor)` works for users (unknown user → `ErrNotFound`).
  - `type LabelUser struct { ID int64; Reachable bool }`; `LabelView.Users []LabelUser` — one per user target of the label, never nil (`[]LabelUser{}`).
  - `UserCard.LabelIDs []int64` (never nil).
  - `Notifier.SendToLabels` / `SendToLabel` / `TestLabel` include reachable users; their outbox rows carry `UserID` = the user and `LabelID` = the label.

- [ ] **Step 1: Write failing tests**

In `label_service_test.go`:
- `TestSetLabelsOnUser`: set `[a]` on a user, then `[b]`; `LabelsOf` is `[b]`; audit has `label_bound` for a and b and `label_unbound` for a, each with target `domain.UserTarget(u.ID)`.
- `TestSetLabelsUnknownUser`: `SetLabels(LabelTarget{TargetUser, 9999}, …)` returns `ErrNotFound`, no bindings written.
- `TestLabelListUsersReachable`: one label on three users — active+linked, active unlinked, disabled+linked; `List` returns `Users` = those three ids with `Reachable` `true, false, false`.
- `TestCardHasLabelIDs`: `UserService.Card` returns the user's label ids; `[]int64{}` when none.

In `notifier_test.go` (use `chatEnv`, `bind`, `pending`, `mkUser`, `link`):
- `TestSendToLabelReachesUsers`: label on an accepted chat and on an active linked user → 2 rows; the user's row has `To.ChatID` = their channel `ChatID`, `UserID` = user id, `LabelID` = label id.
- `TestSendToLabelSkipsUnreachableUsers`: users with status `disabled`, `rejected`, `awaiting_activation`, `pending_approval`, and an active user without a channel, all labelled → 0 rows.
- `TestSendToLabelUserDisabledAfterBinding`: bind an active linked user, then set status `disabled` via the repo → 0 rows.
- `TestSendToLabelsUserOnce`: one user carrying two labels, `SendToLabels([l1, l2])` → 1 row, `LabelID` = l1.
- `TestSendToLabelUserAndTheirChatBoth`: user labelled and a chat labelled with the same label → 2 rows (one to the chat, one private).
- `TestTestLabelUserOnly`: label only on an active linked user → `TestLabel` returns 1 id, no error.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/domain/ -run 'TestSetLabelsOnUser|TestSetLabelsUnknownUser|TestLabelListUsersReachable|TestCardHasLabelIDs|TestSendToLabel|TestTestLabelUserOnly' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- `label_service.go`: `LabelTarget.auditTarget()` returns `UserTarget(t.ID)` for `TargetUser`; `targetExists` handles `TargetUser` via `s.R.Users.Get`. `List` fills `v.Users` for every `TargetUser` target, with `Reachable` from `userReachable`.
- New unexported helper in `notifier.go`: `func userChannel(ctx context.Context, r Repos, userID int64) (Channel, bool, error)` — false unless `Users.Get(...).Status == StatusActive` and `Channels.ListByUser` is non-empty (first channel, as `SendToUser`). `userReachable` in `label_service.go` is `_, ok, err := userChannel(...)`.
- `notifier.go`: `destination` gains `user int64` (0 for chats/topics). `target` resolves `TargetUser` through `userChannel` into `destination{c.Type, Destination{ChatID: c.ChatID}, userID}`. In `SendToLabels`, pass `&d.user` as `userID` to `add` when `d.user != 0`, else nil. Dedup by the whole `destination` (unchanged map).
- `users_service.go`: `UserCard` gains `LabelIDs []int64`; `Card` loads `R.Labels.LabelsOf(ctx, LabelTarget{TargetUser, id})`, normalised to `[]int64{}`.
- Comment on `Notifier.SendToLabels` and the `Label` doc: "every accepted place or active user carrying it".

- [ ] **Step 4: Run all Go tests**

Run: `go test -race ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain
git commit -m "feat(domain): send to labels reaches labelled users"
```

---

### Task 3: HTTP API

**Files:**
- Modify: `internal/httpapi/chats.go`, `internal/httpapi/admin.go`
- Test: `internal/httpapi/chats_test.go`, `internal/httpapi/admin_test.go`

**Interfaces:**
- Consumes: `LabelView.Users`, `UserCard.LabelIDs`, `SetLabels` with `TargetUser` (Task 2).
- Produces (JSON):
  - `PUT /api/v1/admin/users/{id}/labels` body `{"labelIds":[…]}` → 204; unknown user → 404; unknown label → 422 `labels: invalid` (as chats).
  - `GET /api/v1/admin/users/{id}` adds top-level `"labelIds": [..]`.
  - `GET /api/v1/admin/labels` items add `"users": [{"id": 7, "name": "Ann Lee", "reachable": true}]` (always an array). `targets` may contain `{"type":"user","id":7}`.

- [ ] **Step 1: Write failing tests**

- `TestSetUserLabels` (chats_test.go): admin PUTs `[l]` on a user → 204; `GET /admin/users/{id}` has `labelIds == [l]`; `GET /admin/labels` shows `users[0]` with the user's id, their "First Last" name and `reachable` matching whether the fixture user is active with a linked channel; PUT on user 9999 → 404; non-admin → 403.
- `TestAuditNamesForChatsTopicsLabels` (admin_test.go): extend with a `label_bound` on a user; its `targetName` is the user's name.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/httpapi/ -run 'TestSetUserLabels|TestAuditNamesForChatsTopicsLabels' -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- `chats.go`: route `PUT /api/v1/admin/users/{id}/labels` → `h.setLabels(domain.TargetUser)`. `labelDTO` gains `Users []labelUserDTO \`json:"users"\`` with `labelUserDTO{ID int64 \`json:"id"\`; Name string \`json:"name"\`; Reachable bool \`json:"reachable"\`}`. `listLabels` builds a `namer` (`newNamer`) and fills `Name` with `n.userName(ctx, id)`; `toLabel` takes the namer (or a name func) — the other `toLabel` callers (create/update responses) get `Users: []` from the view.
- `admin.go` `writeCard`: add `"labelIds": ids(card.LabelIDs)`.
- If audit naming of `user:N` targets already resolves via `n.userName`, nothing else is needed; otherwise make it.

- [ ] **Step 4: Run all Go tests**

Run: `go test -race ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi
git commit -m "feat(api): edit a user's labels; label lists show users"
```

---

### Task 4: Admin web UI

**Files:**
- Modify: `web/src/api/types.ts`, `web/src/api/chats.ts`, `web/src/lib/chats.ts`, `web/src/pages/admin/LabelsModal.tsx`, `web/src/pages/admin/UserCardPage.tsx`, `web/src/pages/admin/LabelsPage.tsx`, `web/src/pages/admin/FeedPostEditor.tsx`, `web/src/pages/admin/FeedAdminPage.tsx`, `web/src/pages/admin/NotificationsPage.tsx`, `web/src/locales/uk.json`, `web/src/locales/en.json`
- Test: `web/src/pages/admin/UserCardPage.test.tsx`, `web/src/pages/admin/LabelsPage.test.tsx`, `web/src/pages/admin/FeedPostEditor.test.tsx`, `web/src/lib/chats.test.ts` (create)

**Interfaces:**
- Consumes: the JSON from Task 3.
- Produces:
  - `LabelTargetType = 'chat' | 'chat_topic' | 'user'`; `Label.users: { id: number; name: string; reachable: boolean }[]`; `UserCard.labelIds: number[]`.
  - `reachesSomeone(l: Label, chats: Chat[]): boolean` replaces `reachesChat` at all four call sites (`reachesChat || l.users.some(u => u.reachable)`). Remove `reachesChat` once unused.
  - `labelPlaces` returns user names (from `l.users`) for `user` targets.
  - `useSetLabels` path for `user` is `/admin/users/${id}`; on success also invalidate that user's card query.

- [ ] **Step 1: Write failing tests**

- `lib/chats.test.ts`: `reachesSomeone` true for a label whose only target is a reachable user, false when that user is `reachable: false` and no accepted chat; `labelPlaces` returns the user's name for a user target.
- `UserCardPage.test.tsx` `'shows and edits the person's labels'`: card with `labelIds: [1]` shows the label name as a badge on the details tab; clicking "Edit labels" opens the modal; saving sends `PUT /admin/users/{id}/labels` with the chosen ids.
- `LabelsPage.test.tsx` `'lists the people a label hangs on'`: a label with a user target shows the user's name among its places.
- `FeedPostEditor.test.tsx` `'offers a label that reaches only people'`: a label whose only target is a reachable user is offered; one with only an unreachable user is not.

- [ ] **Step 2: Run to see them fail**

Run: `cd web && npx vitest run src/lib/chats.test.ts src/pages/admin/UserCardPage.test.tsx src/pages/admin/LabelsPage.test.tsx src/pages/admin/FeedPostEditor.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

- User card, details tab: a `Card withBorder` titled "Labels" (`admin.user.labels`) with the label badges (reuse the badge look from `ChatsPage` `LabelBadges`; move it to a shared component if used twice) or `admin.user.noLabels`, and an "Edit labels" button (`admin.user.editLabels`) opening `LabelsModal` with `target={{ type: 'user', id }}` and `current={card.labelIds}`. A hint under it (`admin.user.labelsHint`): messages sent to these labels come to this person in the bot.
- `LabelsModal` doc comment: "of a chat, topic or user".
- Copy (uk / en):
  - `admin.user.labels`: "Лейбли" / "Labels"
  - `admin.user.noLabels`: "Лейблів немає" / "No labels"
  - `admin.user.editLabels`: "Змінити лейбли" / "Edit labels"
  - `admin.user.labelsHint`: "Повідомлення, надіслані на ці лейбли, прийдуть цій людині в бот." / "Messages sent to these labels come to this person in the bot."
  - `admin.notifications.noChat` → "Цей лейбл ще не веде ні в прийнятий чат, ні до людини з ботом, тож сповіщення нікуди не підуть." / "This label does not reach an accepted chat or a person with the bot yet, so the notices will go nowhere."
  - `errors.chat_not_accepted` → "Тут поки ніхто не може отримати повідомлення: прийміть чат або перевірте, що в людини підключений бот." / "Nobody here can receive messages yet: accept the chat, or check the person has the bot linked."

- [ ] **Step 4: Run web checks**

Run: `cd web && npm test && npm run lint && npm run typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "feat(web): labels on the user card; label lists count people"
```

---

### Task 5: Whole-branch check

- [ ] **Step 1:** `make test && make lint` — expected PASS.
- [ ] **Step 2:** Run the app (`run` skill / `make run`), in the admin: create label "testers", hang it on yourself from the user card, use "Send test message" on the label — the message arrives in your private chat with the bot; the Labels page lists you under "testers"; the feed post editor offers "testers".
