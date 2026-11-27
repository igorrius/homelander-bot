# M2 Chats, Labels, Outbox and Registration from the Bot — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The bot records every chat and forum topic it is in, admins accept chats and hang labels on them, the core sends messages to labels through a durable outbox (first: new-application notices), and registration can start in the bot with the phone pre-proven.

**Architecture:** Messenger-neutral domain entities (`Chat`, `Topic`, `Label`, `OutboxItem`, `RegistrationToken`) in `internal/domain` with SQLite repositories; the Telegram adapter only translates updates into `ChatEvents` calls and implements `Send`/`LeaveChat`; a `Notifier` enqueues rows inside the caller's transaction and an `OutboxWorker` goroutine delivers them. The web app gains a Chats section (with Labels), a Notifications setting, and token-aware `/register`.

**Tech Stack:** Go 1.27, `modernc.org/sqlite`, `github.com/PaulSonOfLars/gotgbot/v2` v2.0.0-rc.37, `nicksnyder/go-i18n/v2`, `net/http`; React + TypeScript + Vite, Mantine, TanStack Query, `react-i18next`, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-11-27-m2-chats-labels-and-bot-registration-design.md` (approved 2026-11-27). Builds on `docs/superpowers/plans/2026-11-27-m1-backend-and-bot.md` and `docs/superpowers/plans/2026-11-27-m1-web.md`; read their Global Constraints too.

## Global Constraints

- Everything in the M1 plans' Global Constraints still holds (i18n keys in both locales with parity tests, API errors as codes, no real phones/names/chats — test phones `+380500000001`…`+380500000099`, test Telegram ids `1000001`…, test chat ids `-1001000000001`…; `sqlite.DB.Q(ctx)` for every statement; UTC RFC 3339 times; `domain.Clock` in services).
- **One new migration** `internal/storage/sqlite/migrations/0003_chats_labels_outbox.sql` (Task 1); applied migrations `0001`/`0002` are never edited.
- Audit action names follow the existing dotted style: `chat.added`, `chat.auto_left`, `chat.accepted`, `chat.left`, `chat.removed`, `chat.moved`, `topic.renamed`, `label.created`, `label.updated`, `label.deleted`, `label.bound`, `label.unbound`, `notifications.updated`. Targets: `chat:<id>`, `topic:<id>`, `label:<id>`, `notifications`.
- The domain never imports gotgbot; the Telegram package never decides acceptance, labels or retries.
- No phone number in any message sent to a group or channel.
- Registration tokens and their hashes: 32 random bytes, base64url (no padding); stored as HMAC-SHA256 with `APP_SECRET`; never logged; lifetime **1 h**; carried in the URL fragment `#t=`.
- Outbox numbers: worker tick **5 s**; per-chat spacing **≥ 3 s**; backoff **10 s doubling, cap 30 min**; give up **24 h** after `created_at`; delete `sent`/`failed` rows older than **30 days**.
- Chat leave grace: **2 min** (`leave_at`). Telegram's anonymous-group-admin account id: `1087968824` (`GroupAnonymousBot`).
- `allowed_updates` = `["message", "my_chat_member"]` for both polling and webhook.
- Work on a branch, PR into `main`; commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. A group upgraded to a supergroup by a moderator who is not a system admin: Telegram reports "bot added" to the new ID before/after the migration messages — the bot must stay and the chat keep its labels (Task 4 `TestUpgradeByNonAdminKeepsBot`).
2. Telegram redelivers the same update (webhook retry, restart before offset commit): chats, topics and notices must not duplicate (Task 4 `TestChatSeenAndTopicSeenAreIdempotent`, Task 8 `TestSameApplicationNotifiedOnce`).
3. Telegram unreachable for hours: applications still succeed, notices wait and go out later in order (Task 6 `TestWorkerRetriesUntilMessengerRecovers`, Task 8 `TestApplySucceedsWhenMessengerDown`).
4. An admin deletes or moves a label while messages for it are queued: queued rows still go to the destinations resolved at enqueue time; the notification setting is cleared (Task 5 `TestDeleteLabelClearsSettingKeepsQueue`).
5. A bot link opened late, twice, or on another device: the form falls back to an ordinary editable form and a second submit cannot re-link anything (Task 8 `TestTokenReuseIsSilent`, Task 11 `RegisterPage falls back on an invalid token`).

---

### Task 1: Schema and repositories for chats, topics, labels

**Files:**
- Create: `internal/storage/sqlite/migrations/0003_chats_labels_outbox.sql`, `internal/domain/chat.go`, `internal/domain/label.go`, `internal/storage/sqlite/chats.go`, `internal/storage/sqlite/labels.go`, `internal/storage/sqlite/chats_test.go`, `internal/storage/sqlite/labels_test.go`
- Modify: `internal/domain/users_service.go` (`Repos`), `internal/domain/testenv_test.go`, `cmd/homelander/main.go` (repos wiring only)

**Interfaces:**
- Produces:
  ```go
  package domain
  type ChatState string
  const ( ChatNew ChatState = "new"; ChatAccepted ChatState = "accepted"; ChatLeft ChatState = "left" )
  // ChatInfo is what a messenger reports about a chat.
  type ChatInfo struct{ Messenger, ExternalID, Kind, Title string; IsForum bool } // Kind: group|supergroup|channel
  type BotRights struct{ Status string; CanSend bool } // Status: member|administrator|restricted|unknown
  type Chat struct {
      ID int64; Messenger, ExternalID, Kind, Title string; IsForum bool; State ChatState
      BotStatus string; CanSend bool; AddedByExternalID string; AddedByUserID, AcceptedBy *int64
      AcceptedAt, LeaveAt, LeftAt *time.Time; FirstSeenAt, LastSeenAt time.Time
  }
  type Topic struct{ ID, ChatID int64; ExternalID, Title string; TitleSetByAdmin, Gone bool; FirstSeenAt, LastSeenAt time.Time }
  type ChatRepo interface {
      Create(ctx context.Context, c *Chat) error
      Update(ctx context.Context, c *Chat) error
      Get(ctx context.Context, id int64) (Chat, error)
      GetByExternal(ctx context.Context, messenger, externalID string) (Chat, error)
      List(ctx context.Context) ([]Chat, error)            // new first, then accepted, then left; by title
      Delete(ctx context.Context, id int64) error
      DueLeaves(ctx context.Context, now time.Time) ([]Chat, error) // state=new AND leave_at<=now
      CountByState(ctx context.Context, s ChatState) (int, error)
  }
  type TopicRepo interface {
      Create(ctx context.Context, t *Topic) error
      Update(ctx context.Context, t *Topic) error
      Get(ctx context.Context, id int64) (Topic, error)
      GetByExternal(ctx context.Context, chatID int64, externalID string) (Topic, error)
      ListByChat(ctx context.Context, chatID int64) ([]Topic, error)
      MoveToChat(ctx context.Context, fromChatID, toChatID int64) error // skips topics whose external id exists in target
  }
  type Label struct{ ID int64; Name, Description string; CreatedAt, UpdatedAt time.Time }
  const ( TargetChat = "chat"; TargetTopic = "chat_topic" )
  type LabelTarget struct{ Type string; ID int64 }
  type LabelRepo interface {
      Create(ctx context.Context, l *Label) error // ErrConflict on a case-folded duplicate name
      Update(ctx context.Context, l *Label) error
      Get(ctx context.Context, id int64) (Label, error)
      List(ctx context.Context) ([]Label, error)
      Delete(ctx context.Context, id int64) error
      Targets(ctx context.Context, labelID int64) ([]LabelTarget, error)
      LabelsOf(ctx context.Context, t LabelTarget) ([]int64, error)
      SetLabelsOf(ctx context.Context, t LabelTarget, labelIDs []int64) error
      Retarget(ctx context.Context, from, to LabelTarget) error // INSERT OR IGNORE then delete old
      DeleteTarget(ctx context.Context, t LabelTarget) error
  }
  ```
  `Repos` gains `Chats ChatRepo; Topics TopicRepo; Labels LabelRepo` (and in Task 2 `Outbox`, `RegTokens`, `Notifications`); `sqlite.Chats{DB}`, `sqlite.Topics{DB}`, `sqlite.Labels{DB}`.

- [ ] **Step 1: Write the migration** with every M2 table from spec 5.2: `chats` (incl. `leave_at`), `chat_topics`, `labels` (plus a `name_key TEXT NOT NULL UNIQUE` holding the application-folded name, as `users.search_text` does), `label_bindings`, `outbox` (index on `(state, next_attempt_at)`), `registration_tokens` (unique `token_hash`, index on `(channel_type, external_id)`). All `STRICT`, `CHECK` on enum columns, `chat_topics.chat_id` `ON DELETE CASCADE`.
- [ ] **Step 2: Write failing tests** `TestChatsCRUD` (create/get/getByExternal/update round-trips every field incl. nil times), `TestChatsListOrder`, `TestChatsDueLeaves` (only `new` with past `leave_at`), `TestTopicsUniquePerChat`, `TestTopicsMoveToChat` (a duplicate external id in target is left behind), `TestLabelNameUniqueCaseFolded` (`"Журнал"` vs `"журнал"` → `ErrConflict`), `TestLabelBindings` (SetLabelsOf replaces; Targets lists; Retarget merges without duplicates; DeleteTarget), `TestLabelDeleteCascadesBindings`.
- [ ] **Step 3: Run** `go test ./internal/storage/sqlite/` — Expected: FAIL (types missing).
- [ ] **Step 4: Implement** domain types and repos; name folding with the same helper the users search uses.
- [ ] **Step 5: Run** `go test -race ./internal/storage/sqlite/ ./internal/domain/` — Expected: PASS (incl. `TestEmbeddedMigrationsApply`).
- [ ] **Step 6: Commit** `feat: chats, topics and labels schema and repositories`

### Task 2: Repositories for outbox, registration tokens, notification settings

**Files:**
- Create: `internal/domain/outbox.go`, `internal/domain/regtoken.go`, `internal/storage/sqlite/outbox.go`, `internal/storage/sqlite/regtokens.go`, `internal/storage/sqlite/notifications.go`, matching `_test.go` files
- Modify: `internal/domain/users_service.go` (`Repos`), `internal/domain/testenv_test.go`, `cmd/homelander/main.go`

**Interfaces:**
- Consumes: Task 1 migration.
- Produces:
  ```go
  package domain
  type Destination struct{ ChatID, ThreadID string }
  type LinkButton struct{ Text, URL string }
  type OutMessage struct{ Text string; Buttons []LinkButton }
  type OutboxState string
  const ( OutboxPending OutboxState = "pending"; OutboxSent OutboxState = "sent"; OutboxFailed OutboxState = "failed" )
  type OutboxItem struct {
      ID int64; Messenger string; To Destination; Msg OutMessage; Origin string
      LabelID, UserID *int64; State OutboxState; Attempts int; NextAttemptAt time.Time
      LastError string; CreatedAt time.Time; SentAt *time.Time
  }
  type OutboxRepo interface {
      Add(ctx context.Context, it *OutboxItem) error
      Get(ctx context.Context, id int64) (OutboxItem, error)
      Due(ctx context.Context, now time.Time, limit int) ([]OutboxItem, error) // pending, next_attempt_at<=now, oldest id first
      Update(ctx context.Context, it *OutboxItem) error
      RepointChat(ctx context.Context, messenger, oldChatID, newChatID string) error // pending rows only
      DeleteFinishedBefore(ctx context.Context, t time.Time) (int, error)
  }
  type RegistrationToken struct {
      ID int64; TokenHash []byte; ChannelType, ExternalID, ChatID, Username, Phone string
      ExpiresAt time.Time; UsedAt *time.Time; CreatedAt time.Time
  }
  type RegTokenRepo interface {
      Create(ctx context.Context, t *RegistrationToken) error
      GetByHash(ctx context.Context, hash []byte) (RegistrationToken, error)
      MarkUsed(ctx context.Context, id int64, at time.Time) error
      InvalidateFor(ctx context.Context, channelType, externalID string, at time.Time) error // sets used_at on unused ones
  }
  type NotificationSettings struct{ ApplicationsLabelID *int64 }
  type NotificationRepo interface {
      Get(ctx context.Context) (NotificationSettings, error) // zero value when unset
      Save(ctx context.Context, s NotificationSettings) error // settings key "notifications", JSON {"applicationsLabelId": n|null}
  }
  ```
  `Repos` gains `Outbox OutboxRepo; RegTokens RegTokenRepo; Notifications NotificationRepo`.

- [ ] **Step 1: Write failing tests** `TestOutboxRoundTrip` (payload with buttons survives), `TestOutboxDueOrderAndFilter`, `TestOutboxRepointChatPendingOnly`, `TestOutboxDeleteFinishedBefore` (pending rows never deleted), `TestRegTokenGetByHashAndMarkUsed`, `TestRegTokenInvalidateFor`, `TestNotificationsDefaultAndSave`.
- [ ] **Step 2: Run** `go test ./internal/storage/sqlite/` — Expected: FAIL.
- [ ] **Step 3: Implement** the three repos (payload as JSON `{"text":…, "buttons":[{"text":…,"url":…}]}`).
- [ ] **Step 4: Run** `go test -race ./internal/storage/sqlite/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: outbox, registration token and notification settings storage`

### Task 3: Messenger boundary — Send, LeaveChat, error classes, Telegram implementation

**Files:**
- Modify: `internal/domain/messenger.go`, `internal/domain/fakemessenger_test.go`, `internal/messenger/telegram/telegram.go`, `internal/messenger/telegram/telegram_test.go`
- Create: `internal/messenger/telegram/send.go`, `internal/messenger/telegram/send_test.go`

**Interfaces:**
- Consumes: `Destination`, `OutMessage` (Task 2).
- Produces:
  ```go
  package domain
  type Messenger interface {
      Type() string
      SendText(ctx context.Context, chatID, text string) error
      Send(ctx context.Context, to Destination, m OutMessage) error // failure: *SendError
      LeaveChat(ctx context.Context, chatID string) error           // nil when the bot is already not there
      BotURL() string
  }
  type SendErrorKind string
  const ( SendRetry SendErrorKind = "retry"; SendChatGone SendErrorKind = "chat_gone"; SendThreadGone SendErrorKind = "thread_gone"; SendChatMoved SendErrorKind = "chat_moved"; SendPermanent SendErrorKind = "permanent" )
  type SendError struct{ Kind SendErrorKind; RetryAfter time.Duration; NewChatID string; Err error }
  func (e *SendError) Error() string
  func (e *SendError) Unwrap() error
  // Messengers looks a messenger up by type.
  type Messengers map[string]Messenger
  func (m Messengers) Get(typ string) (Messenger, error) // ErrNotFound for an unknown type
  ```
  `telegram.API` gains `LeaveChat(chatId int64, opts *gotgbot.LeaveChatOpts) (bool, error)` and `GetChatMember(chatId, userId int64, opts *gotgbot.GetChatMemberOpts) (gotgbot.ChatMember, error)`. `fakeMessenger` gets `Send`/`LeaveChat` that record calls and can be scripted to return a given error per call.

- [ ] **Step 1: Write failing tests** in `send_test.go`: `TestSendPlainAndThread` (`ThreadID "7"` → `MessageThreadId: 7`), `TestSendButtonsAsInlineKeyboard` (one URL button per row), `TestClassifyTelegramErrors` — table over `*gotgbot.TelegramError`: 429 with `RetryAfter 5` → `SendRetry, 5s`; 500 / network error → `SendRetry`; 403 → `SendChatGone`; 400 with `ResponseParams.MigrateToChatId -100123` → `SendChatMoved, "-100123"`; 400 description containing `"message thread not found"` → `SendThreadGone`; 400 `"chat not found"` → `SendChatGone`; other 400 → `SendPermanent`. `TestLeaveChatAlreadyGoneIsNil` (403/400 "chat not found" → nil).
- [ ] **Step 2: Run** `go test ./internal/messenger/telegram/` — Expected: FAIL.
- [ ] **Step 3: Implement** `Send`, `LeaveChat`, `classify(err error) *domain.SendError` in `send.go`; extend interfaces and fakes so `go build ./...` passes.
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: messenger Send and LeaveChat with classified errors`

### Task 4: ChatService — lifecycle, grace leave, migration, topics

**Files:**
- Create: `internal/domain/chat_service.go`, `internal/domain/chat_service_test.go`
- Modify: `internal/domain/audit.go` (actions from Global Constraints)

**Interfaces:**
- Consumes: Tasks 1–3.
- Produces:
  ```go
  package domain
  const AnonymousAdminExternalID = "1087968824"
  type ChatService struct {
      R Repos; Now Clock; Messengers Messengers; Log *slog.Logger
      LeaveGrace time.Duration // 2 min
  }
  // ChatEvents is what a messenger adapter calls; ChatService implements it.
  type ChatEvents interface {
      BotAdded(ctx context.Context, c ChatInfo, addedByExternalID string, r BotRights) error
      BotRightsChanged(ctx context.Context, c ChatInfo, r BotRights) error
      BotRemoved(ctx context.Context, c ChatInfo) error
      ChatSeen(ctx context.Context, c ChatInfo) (created bool, err error)
      TopicSeen(ctx context.Context, c ChatInfo, threadID, name string) error // name "" = unknown
      ChatMoved(ctx context.Context, messenger, oldID, newID string) error
      IsAccepted(ctx context.Context, messenger, externalID string) (bool, error)
  }
  func (s *ChatService) Accept(ctx context.Context, id int64, a Actor) error   // new → accepted (clears leave_at); other states → ErrConflict
  func (s *ChatService) Leave(ctx context.Context, id int64, a Actor) error    // new|accepted → left; calls Messenger.LeaveChat first
  func (s *ChatService) RenameTopic(ctx context.Context, id int64, title string, a Actor) error // 1..128 chars; sets TitleSetByAdmin
  func (s *ChatService) LeaveDue(ctx context.Context) (int, error)            // processes ChatRepo.DueLeaves
  func (s *ChatService) MarkGone(ctx context.Context, messenger, chatID string) error         // used by the worker on SendChatGone
  func (s *ChatService) MarkTopicGone(ctx context.Context, messenger, chatID, threadID string) error
  type ChatView struct{ Chat; LabelIDs []int64; Topics []TopicView }
  type TopicView struct{ Topic; LabelIDs []int64 }
  func (s *ChatService) List(ctx context.Context) ([]ChatView, error)
  func (s *ChatService) NewCount(ctx context.Context) (int, error)
  ```
  Behaviour (spec 5.3): `BotAdded` by the linked Telegram account of an `active` admin → `accepted` (`AddedByUserID`, `AcceptedBy` set; audit `chat.added`+`chat.accepted`); by `AnonymousAdminExternalID` → `new`, no `leave_at`; by anyone else → `new` with `LeaveAt = now+LeaveGrace` (audit `chat.added` with `addedBy`). A known `left` chat re-added by an admin → `accepted`, labels kept; by others → `new`+`leave_at`. `LeaveDue` calls `LeaveChat`, sets `left`, audits `chat.auto_left`. `BotRemoved` → `left` (`chat.removed`). `ChatSeen` creates a `new` chat (no `leave_at`) or updates title/kind/is_forum/last_seen; a `left` chat that is seen again (the bot is evidently there) becomes `new`. `TopicSeen` creates/updates; a non-empty `name` overwrites `Title` and clears `TitleSetByAdmin`. `ChatMoved` per spec 5.3 (rename-in-place, or merge into an existing row: topics via `MoveToChat`, bindings via `Retarget`, `accepted` if either was, `leave_at` cleared, outbox `RepointChat`, old row deleted; audit `chat.moved`); second call is a no-op. `Accept` clears `leave_at`.

- [ ] **Step 1: Write failing tests** `TestAddedByAdminIsAccepted`, `TestAddedByStrangerLeavesAfterGrace` (before 2 min `LeaveDue` does nothing; after → `LeaveChat` called, state `left`, audit `chat.auto_left`), `TestAddedByAnonymousAdminWaitsAsNew`, `TestReAddedByAdminKeepsLabels`, `TestSeenWithoutAddIsNew`, `TestAcceptCancelsGrace`, `TestLeaveCallsMessengerThenSetsLeft`, `TestLeaveWhenAlreadyGone`, `TestBotRemoved`, `TestUpgradeByNonAdminKeepsBot` (old accepted chat with label; `BotAdded` for new id by a stranger, then `ChatMoved(old,new)` → one accepted chat with the new id, label kept, no `leave_at`, `LeaveDue` after 5 min leaves nothing), `TestChatMovedEitherOrderAndTwice`, `TestChatMovedRepointsPendingOutbox`, `TestTopicNaming` (unknown → named by created → admin rename → Telegram edit overwrites), `TestChatSeenAndTopicSeenAreIdempotent`, `TestIsAcceptedOnlyForAccepted`.
- [ ] **Step 2: Run** `go test ./internal/domain/ -run 'Chat|Topic|Added|Upgrade|Leave|Accept|Seen|Bot'` — Expected: FAIL.
- [ ] **Step 3: Implement** `chat_service.go` (network call `LeaveChat` outside the transaction; state written after it).
- [ ] **Step 4: Run** `go test -race ./internal/domain/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: chat lifecycle with grace leave, migration and topics`

### Task 5: LabelService and notification settings

**Files:**
- Create: `internal/domain/label_service.go`, `internal/domain/label_service_test.go`

**Interfaces:**
- Consumes: Tasks 1, 2, 4.
- Produces:
  ```go
  package domain
  type LabelService struct{ R Repos; Now Clock }
  type LabelView struct{ Label; Targets []LabelTarget; UsedByApplications bool }
  func (s *LabelService) List(ctx context.Context) ([]LabelView, error)
  func (s *LabelService) Create(ctx context.Context, name, description string, a Actor) (Label, error) // name trimmed 1..40, description ≤200; ValidationError{"name":"required"|"too_long"|"taken"}
  func (s *LabelService) Update(ctx context.Context, id int64, name, description string, a Actor) (Label, error)
  func (s *LabelService) Delete(ctx context.Context, id int64, a Actor) error // clears NotificationSettings.ApplicationsLabelID when it points here (audit notifications.updated)
  func (s *LabelService) SetLabels(ctx context.Context, t LabelTarget, labelIDs []int64, a Actor) error // target must exist; unknown label → ValidationError{"labels":"invalid"}; audits bound/unbound diffs
  func (s *LabelService) Notifications(ctx context.Context) (NotificationSettings, error)
  func (s *LabelService) SaveNotifications(ctx context.Context, n NotificationSettings, a Actor) error // unknown label → ValidationError{"applicationsLabelId":"invalid"}
  ```

- [ ] **Step 1: Write failing tests** `TestLabelCreateValidation` (empty, 41 chars, duplicate in other case → `taken`), `TestLabelRename`, `TestSetLabelsOnChatAndTopic` (audit diff entries), `TestSetLabelsUnknownTarget` (`ErrNotFound`), `TestDeleteLabelClearsSettingKeepsQueue` (an outbox row queued for the label stays `pending` and keeps its destination), `TestSaveNotificationsValidates`.
- [ ] **Step 2: Run** `go test ./internal/domain/ -run 'Label|Notifications'` — Expected: FAIL.
- [ ] **Step 3: Implement** `label_service.go`.
- [ ] **Step 4: Run** `go test -race ./internal/domain/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: labels and notification settings`

### Task 6: Notifier and OutboxWorker

**Files:**
- Create: `internal/domain/notifier.go`, `internal/domain/outbox_worker.go`, `internal/domain/notifier_test.go`, `internal/domain/outbox_worker_test.go`
- Modify: `locales/en.json`, `locales/uk.json` (key `notify.test`)

**Interfaces:**
- Consumes: Tasks 1–5.
- Produces:
  ```go
  package domain
  type Notifier struct {
      R Repos; Now Clock; I18n Translator
      DefaultLocale func(ctx context.Context) string // site setting, else config default
      Wake func() // non-blocking signal to the worker; may be nil
  }
  // SendToLabel queues one row per accepted destination carrying the label; returns the row count. Call inside the caller's tx.
  func (n *Notifier) SendToLabel(ctx context.Context, labelID int64, m OutMessage, origin string) (int, error)
  // SendToUser queues one row to the user's linked channel; false when the user has none.
  func (n *Notifier) SendToUser(ctx context.Context, userID int64, m OutMessage, origin string) (bool, error)
  // Test* queue notify.test in the default locale; ErrConflict ("chat_not_accepted") when nothing would receive it.
  func (n *Notifier) TestChat(ctx context.Context, chatID int64) ([]int64, error)
  func (n *Notifier) TestTopic(ctx context.Context, topicID int64) ([]int64, error)
  func (n *Notifier) TestLabel(ctx context.Context, labelID int64) ([]int64, error)
  func (n *Notifier) Clean(ctx context.Context) (int, error) // DeleteFinishedBefore(now-30d)
  var ErrChatNotAccepted = errors.New("chat_not_accepted")

  type OutboxWorker struct {
      R Repos; Now Clock; Messengers Messengers; Chats *ChatService; Log *slog.Logger
      Tick, Spacing, MaxAge, BackoffMin, BackoffMax time.Duration // 5s, 3s, 24h, 10s, 30m
      wake chan struct{}
  }
  func NewOutboxWorker(...) *OutboxWorker // or a constructor taking the struct; exposes Wake() func
  func (w *OutboxWorker) Wake()
  func (w *OutboxWorker) Run(ctx context.Context)
  // RunOnce sends every due row it may send now (spacing respected) and returns how many it attempted.
  func (w *OutboxWorker) RunOnce(ctx context.Context) (int, error)
  ```
  Destination resolution: chat targets → `{ChatID: chat.ExternalID}`; topic targets → `{ChatID, ThreadID: topic.ExternalID}`; skip non-accepted chats and gone topics; dedupe equal destinations. `SendToUser` uses `channels.chat_id` of the user's channel and the channel type as messenger. Failure handling exactly as spec 5.4; `Attempts` incremented on every try; `LastError` holds `err.Error()` truncated to 500 chars.

- [ ] **Step 1: Write failing tests** `TestSendToLabelResolvesAcceptedOnly` (new/left chats and gone topics skipped, duplicate binding deduped, empty label → 0), `TestSendToUserNoChannel`, `TestTestChatNotAccepted`, `TestWorkerSendsAndMarksSent`, `TestWorkerRetryBackoff` (10 s, 20 s, 40 s … capped 30 min; `RetryAfter 60s` wins over 10 s), `TestWorkerGivesUpAfter24h`, `TestWorkerChatMovedRetriesAtNewID`, `TestWorkerChatGoneMarksChatLeft`, `TestWorkerThreadGoneMarksTopic`, `TestWorkerPermanentFails`, `TestWorkerSpacesSendsPerChat` (two rows same chat → second waits ≥3 s on the fake clock; another chat not delayed), `TestWorkerRetriesUntilMessengerRecovers`, `TestNotifierCleanKeepsPending`.
- [ ] **Step 2: Run** `go test ./internal/domain/ -run 'SendTo|Test(Chat|Topic|Label)|Worker|Notifier'` — Expected: FAIL.
- [ ] **Step 3: Implement** notifier and worker; add `notify.test` — en: `✅ Test message. Messages for this place will arrive here.`; uk: `✅ Тестове повідомлення. Повідомлення для цього місця приходитимуть сюди.`
- [ ] **Step 4: Run** `go test -race ./internal/domain/ ./internal/i18n/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: notifier and durable outbox worker`

### Task 7: Telegram adapter — chat updates in, accepted chats to the sink

**Files:**
- Modify: `internal/messenger/telegram/telegram.go`, `internal/messenger/telegram/updates.go`, `internal/messenger/telegram/webhook.go` (nothing but the shared `allowedUpdates`), `internal/messenger/telegram/telegram_test.go`
- Create: `internal/messenger/telegram/chats.go`, `internal/messenger/telegram/chats_test.go`, `internal/messenger/telegram/testdata/*.json` (synthetic update fixtures)

**Interfaces:**
- Consumes: `domain.ChatEvents` (Task 4).
- Produces: `telegram.NewWithLocales(api, act, chats domain.ChatEvents, sink, tr, locales, publicURL, log)` (new `chats` parameter; `New` likewise); `allowedUpdates = []string{"message", "my_chat_member"}`.

Mapping (spec 5.1): `my_chat_member` in a non-private chat — old status `left`/`kicked` (or restricted with `IsMember=false`) and new status `member`/`administrator`/`restricted` with `IsMember` → `BotAdded(info, from.Id, rights)`; both present → `BotRightsChanged`; new `left`/`kicked` → `BotRemoved`. `CanSend`: administrator → true for groups, `CanPostMessages` for channels; member → true; restricted → `CanSendMessages` (read via `MergeChatMember()`). Group/supergroup `message`: `MigrateToChatId` → `ChatMoved(chat.Id, migrateTo)` and stop; `MigrateFromChatId` → `ChatMoved(migrateFrom, chat.Id)` and stop; else `ChatSeen`; if it returned `created`, call `GetChatMember(chat, botID)` and `BotRightsChanged` (failure → log, rights stay `unknown`); if `IsTopicMessage`: `TopicSeen(threadID, name)` where name comes from `ForumTopicCreated`, `ForumTopicEdited.Name`, or `ReplyToMessage.ForumTopicCreated.Name`; then, only if `IsAccepted`, `sink.OnGroupMessage`. Private-chat `my_chat_member` ignored. `Kind`: Telegram chat type (`group`/`supergroup`/`channel`).

- [ ] **Step 1: Write failing tests** (fixtures with synthetic ids): `TestAllowedUpdates` (polling `GetUpdates` opts and `SetWebhook` opts both carry the two types), `TestMyChatMemberAdded`, `TestMyChatMemberPromoted`, `TestMyChatMemberRemoved`, `TestMyChatMemberChannelAdmin`, `TestPrivateMyChatMemberIgnored`, `TestGroupMessageSeenAndRightsFetchedOnce` (second message → no `GetChatMember`), `TestTopicNameFromCreatedEditedAndReply`, `TestMigrateMessages` (both directions, no `ChatSeen`), `TestSinkOnlyForAcceptedChats`.
- [ ] **Step 2: Run** `go test ./internal/messenger/telegram/` — Expected: FAIL.
- [ ] **Step 3: Implement** `chats.go`; route from `HandleUpdate` (`u.MyChatMember` before `u.Message`).
- [ ] **Step 4: Run** `go test -race ./internal/messenger/telegram/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: telegram adapter records chats, topics and migrations`

### Task 8: Registration from the bot, approval activation, notices

**Files:**
- Modify: `internal/domain/auth_service.go`, `internal/domain/users_service.go`, `internal/domain/errors.go`, `internal/domain/auth_service_test.go`, `internal/domain/users_service_test.go`, `internal/messenger/telegram/telegram.go`, `internal/messenger/telegram/telegram_test.go`, `locales/en.json`, `locales/uk.json`
- Create: `internal/domain/regtoken_service.go` (token issue/verify helpers), `internal/domain/registration_test.go`

**Interfaces:**
- Consumes: Tasks 2, 5, 6.
- Produces:
  ```go
  package domain
  const ( ApplyLink ActivationResult = "apply_link"; PendingLinked ActivationResult = "pending_linked" )
  type Activation struct{ Result ActivationResult; User User; RegistrationURL string } // URL set only for ApplyLink
  // ActivateChannel now returns Activation (telegram.Activator updated accordingly).
  func (s *AuthService) ActivateChannel(ctx context.Context, typ, externalID, chatID, username, verifiedPhone string) (Activation, error)
  var ErrTokenInvalid = errors.New("token_invalid")
  // Application gains: Token string
  func (s *UserService) PrefillRegistration(ctx context.Context, token string) (phone string, err error) // ErrTokenInvalid
  ```
  `AuthService` gains `PublicURL string`, `TokenTTL time.Duration` (1 h). `UserService` gains `Secret []byte`, `Notifier *Notifier`, `I18n Translator`, `PublicURL string`, `DefaultLocale func(ctx) string`. Token URL: `PublicURL + "/register#t=" + token`.

Behaviour: the decision order of spec 5.5 replaces the early return in `ActivateChannel` (rate limit and bootstrap unchanged). Issuing a link calls `RegTokens.InvalidateFor` then `Create`. `Apply` with `Token`: inside the existing transaction, hash → `GetByHash`; missing/used/expired → `ErrTokenInvalid`; the token's phone replaces the form phone; `MarkUsed`; existing M1 branches; for a created or reused user, link the channel per the decision order re-checked (move from a rejected other user with `channel.unlinked`/`channel.linked` audits; otherwise no link); audit `user.applied` details gain `"via":"bot"|"site"`. When a user is created or renewed and `Notifications.ApplicationsLabelID` is set → `Notifier.SendToLabel` with `notify.application` (default locale; residency status name in that locale; `Via` from `notify.via.bot|site`; `Pending` = pending count after insert) and a button `notify.application.button` → `PublicURL + "/admin/applications"`. `review`: approve + channel → `active`, `ActivatedAt`, audits `user.approved` then new `user.activated`; `SendToUser(notify.approved, button notify.approved.button → PublicURL+"/login")` in the user's locale; reject + channel → `SendToUser(notify.rejected)`. The existing phone-edit rule (`"phone":"linked"` when a channel exists) already covers pending users — add a test only.

Bot copy (en / uk):
- `bot.apply.offer`: `You are not registered yet. Fill in the application — your number is already in it. The link works for 1 hour.` / `Вас ще немає в системі. Заповніть заявку — ваш номер уже в ній. Посилання діє 1 годину.`; `bot.apply.button`: `📝 Fill in the application` / `📝 Заповнити заявку`; when `PublicURL` is not `https`, the URL is appended to the text on a new line instead of a button.
- `bot.pendingLinked`: `Your application is waiting for an administrator. I will write to you as soon as it is decided.` / `Вашу заявку ще розглядає адміністратор. Я напишу вам, щойно буде рішення.`
- `bot.help`: `To connect your account or apply for registration, send /start and share your phone number.` / `Щоб підключити акаунт або подати заявку на реєстрацію, надішліть /start і поділіться номером телефону.`
- `notify.approved`: `Your application is approved! You can now log in.` / `Вашу заявку схвалено! Тепер ви можете увійти.`; `notify.approved.button`: `Log in` / `Увійти`
- `notify.rejected`: `Your application was not approved. For questions, contact the administrators.` / `Вашу заявку не схвалено. З питань звертайтеся до адміністраторів.`
- `notify.application`: `📝 New application\n{{.Name}} — house {{.House}}, apt {{.Apartment}}, {{.Status}}\nVia: {{.Via}} · Waiting: {{.Pending}}` / `📝 Нова заявка\n{{.Name}} — будинок {{.House}}, кв. {{.Apartment}}, {{.Status}}\nЧерез: {{.Via}} · Очікують: {{.Pending}}`; `notify.via.bot` `bot`/`бот`; `notify.via.site` `site`/`сайт`; `notify.application.button` `Open applications` / `Відкрити заявки`

- [ ] **Step 1: Write failing tests** in `registration_test.go`: one test per branch of the decision order (`TestBotUnknownPhoneGetsLink`, `TestBotRejectedGetsLink`, `TestBotRejectedWithOtherAccountNeutral`, `TestBotPendingGetsLinked`, `TestBotPendingWithOtherAccountNeutral`, `TestBotAccountOfRejectedOtherPhoneIsFree`, `TestBotAccountOfActiveOtherPhoneNeutral`, `TestBotAwaitingActivationActivates`, `TestBotDisabledNeutral`), `TestNewLinkInvalidatesOld`, `TestPrefill` (valid → phone; expired/used/unknown → `ErrTokenInvalid`), `TestApplyWithTokenUsesTokenPhoneAndLinks`, `TestTokenReuseIsSilent` (second submit → nil, nothing changes), `TestApplyWithTokenMovesLinkFromRejectedOther`, `TestApplyWithTokenAfterPhoneBecameActive` (silent, no link), `TestApproveWithChannelActivatesAndNotifies` (user's locale), `TestApproveWithoutChannelAwaitsActivation`, `TestRejectWithChannelNotifies`, `TestApplicationNoticeQueuedWithoutPhone` (text has no digits of the phone; default locale), `TestSameApplicationNotifiedOnce` (duplicate phone apply → no second row), `TestNoNoticeWhenSettingUnset`, `TestApplySucceedsWhenMessengerDown` (fake messenger errors; Apply nil; row pending), `TestPhoneEditRefusedForPendingWithChannel`. In `telegram_test.go`: `TestContactUnknownGetsApplyButton`, `TestApplyLinkAsTextForHTTP`, `TestContactPendingLinkedReply`.
- [ ] **Step 2: Run** `go test ./internal/domain/ ./internal/messenger/telegram/` — Expected: FAIL.
- [ ] **Step 3: Implement** the domain changes, the adapter's replies and the locale keys.
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS (M1 activation tests updated only where the spec changed the answer: unknown and rejected phones).
- [ ] **Step 5: Commit** `feat: registration from the bot, activation on approval, application notices`

### Task 9: HTTP API for chats, labels, notifications, tokens; wiring

**Files:**
- Create: `internal/httpapi/chats.go`, `internal/httpapi/chats_test.go`
- Modify: `internal/httpapi/admin.go` (routes, summary), `internal/httpapi/public.go` (`API`, prefill, register token), `internal/httpapi/dto.go`, `internal/httpapi/errors.go` (`token_invalid`, `chat_not_accepted`), `internal/httpapi/public_test.go`, `internal/httpapi/apitest_test.go`, `cmd/homelander/main.go`, `cmd/homelander/main_test.go`

**Interfaces:**
- Consumes: Tasks 4–8.
- Produces (JSON, camelCase, as in M1):
  - `GET /api/v1/admin/chats` → `[{id, messenger, kind, title, isForum, state, botStatus, canSend, addedBy: {externalId, userId|null}, leaveAt|null, firstSeenAt, lastSeenAt, labelIds, topics:[{id, externalId, title, titleSetByAdmin, gone, lastSeenAt, labelIds}]}]`
  - `POST /api/v1/admin/chats/{id}/accept`, `POST /api/v1/admin/chats/{id}/leave` → 204
  - `PUT /api/v1/admin/chats/{id}/labels`, `PUT /api/v1/admin/topics/{id}/labels` body `{labelIds:[…]}` → 204
  - `PATCH /api/v1/admin/topics/{id}` body `{title}` → 204
  - `POST /api/v1/admin/chats/{id}/test`, `/topics/{id}/test`, `/labels/{id}/test` → `{outboxIds:[…]}`; `GET /api/v1/admin/outbox/{id}` → `{state, lastError}`
  - `GET /api/v1/admin/labels` → `[{id, name, description, targets:[{type, id}], usedByApplications}]`; `POST` `{name, description}` → label; `PATCH /labels/{id}`; `DELETE /labels/{id}` → 204
  - `GET/PUT /api/v1/admin/settings/notifications` `{applicationsLabelId|null}`
  - `GET /api/v1/admin/summary` gains `newChats`
  - Public `POST /api/v1/register/prefill` `{token}` → `{phone}` or 422 `token_invalid`; `POST /api/v1/register` accepts optional `token`
  - `httpapi.API` gains `Chats *domain.ChatService`, `Labels *domain.LabelService`, `Notifier *domain.Notifier`.

Wiring in `main.go`: build `Messengers{telegram.Type: bot}`, `ChatService`, `LabelService`, `Notifier` (DefaultLocale = site setting else `cfg.DefaultLocale`; `Wake = worker.Wake`), `OutboxWorker` goroutine, pass `chats` to `telegram.NewWithLocales`; the hourly cleanup loop also runs `Notifier.Clean`; a 30 s ticker runs `ChatService.LeaveDue`.

- [ ] **Step 1: Write failing tests** `TestChatsEndpointsAdminOnly` (resident → 403 on each new route), `TestListChatsShape`, `TestAcceptLeave`, `TestSetLabelsValidation`, `TestTestSendNotAccepted` (409 `chat_not_accepted`), `TestOutboxStatus`, `TestLabelsCRUDAndTaken` (422 `fields.name = taken`), `TestNotificationsSettings`, `TestSummaryNewChats`, `TestPrefillValidAndInvalid`, `TestRegisterWithToken`.
- [ ] **Step 2: Run** `go test ./internal/httpapi/` — Expected: FAIL.
- [ ] **Step 3: Implement** handlers, DTOs and wiring.
- [ ] **Step 4: Run** `go test -race ./...` and `go vet ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: admin API for chats, labels and notifications; wire the outbox`

### Task 10: Web — Chats and Labels

**Files:**
- Create: `web/src/api/chats.ts`, `web/src/pages/admin/ChatsPage.tsx`, `web/src/pages/admin/ChatsPage.test.tsx`, `web/src/pages/admin/LabelsPage.tsx`, `web/src/pages/admin/LabelsPage.test.tsx`, `web/src/pages/admin/LabelPicker.tsx`, `web/src/pages/admin/TestSendButton.tsx`
- Modify: `web/src/App.tsx`, `web/src/pages/admin/AdminLayout.tsx` (+ test), `web/src/api/admin.ts` (summary type), `web/src/api/types.ts`, `web/src/test/fixtures.ts`, `web/src/locales/en.json`, `web/src/locales/uk.json`

**Interfaces:**
- Consumes: Task 9 endpoints.
- Produces: routes `/admin/chats` (ChatsPage) and `/admin/labels` (LabelsPage), shown as two tabs of one "Chats" section; hooks `useChats()`, `useLabels()`, `useNotifications()` and mutations in `api/chats.ts`; `<LabelPicker value={number[]} onChange>` (Mantine `MultiSelect` with "Create label “…”" option calling `POST /admin/labels`); `<TestSendButton target={{type:'chat'|'topic'|'label', id}} />` (posts, then polls `GET /admin/outbox/{id}` every 1 s up to 30 s, shows sent / failed + reason / still queued).

Layout per spec 5.8: `new` chats first with the hint text and Accept / Leave (Leave behind `ConfirmButton`); accepted chats with topics indented under them, Test / Rename (topic) / Leave; `left` chats in a collapsed "Left chats" block showing who added them; `can_send=false` warning; empty state with the two ways a chat appears and a link to `docs/telegram-bot-setup.md` on GitHub; a `new`-chat counter badge on the menu item from `summary.newChats`. Menu placement: if `feat/admin-overview-nav` is merged (menu groups exist), add "Chats" to the **Community** group; otherwise add a flat item after Houses.

- [ ] **Step 1: Write failing tests** `ChatsPage lists new chats first and accepts one`, `ChatsPage leaves a chat after confirmation`, `ChatsPage shows topics under their forum and renames an unnamed topic`, `ChatsPage edits labels inline and creates a label`, `ChatsPage shows the empty state`, `ChatsPage warns when the bot cannot send`, `TestSendButton reports a failure reason`, `LabelsPage creates, renames and refuses a taken name`, `LabelsPage delete dialog names the chats and the setting that lose the label`, `AdminLayout shows the new chats badge`.
- [ ] **Step 2: Run** `cd web && npm test -- src/pages/admin/ChatsPage.test.tsx src/pages/admin/LabelsPage.test.tsx src/pages/admin/AdminLayout.test.tsx` — Expected: FAIL.
- [ ] **Step 3: Implement** pages, components, hooks and locale keys (uk + en).
- [ ] **Step 4: Run** `cd web && npm run lint && npm run typecheck && npm test` — Expected: PASS (incl. locale parity).
- [ ] **Step 5: Commit** `feat(web): chats and labels admin`

### Task 11: Web — Notifications setting and token-aware registration

**Files:**
- Create: `web/src/pages/admin/NotificationsPage.tsx`, `web/src/pages/admin/NotificationsPage.test.tsx`
- Modify: `web/src/App.tsx`, `web/src/pages/admin/AdminLayout.tsx`, `web/src/pages/RegisterPage.tsx`, `web/src/pages/RegisterPage.test.tsx`, `web/src/api/public.ts`, `web/src/locales/en.json`, `web/src/locales/uk.json`

**Interfaces:**
- Consumes: Task 9 (`/admin/settings/notifications`, `/register/prefill`, `token` on `/register`), Task 10 (`useLabels`).
- Produces: route `/admin/notifications` (under **Settings** when the menu groups exist, otherwise a flat item after Site) with one row "New applications → label" (`Select`, clearable = off, hint when the chosen label hangs on no accepted chat); `RegisterPage` reads `location.hash` `#t=…`, removes it from the address bar (`history.replaceState`), calls prefill once, locks the phone field with the note "Confirmed in Telegram", sends `token` with the form; on `token_invalid` (prefill or submit) shows "This link has expired — get a new one in the bot" with the bot button, unlocks the phone, drops the token.

- [ ] **Step 1: Write failing tests** `NotificationsPage saves and clears the label`, `NotificationsPage warns when the label reaches no chat`, `RegisterPage locks the phone from a valid token and sends the token`, `RegisterPage falls back on an invalid token`, `RegisterPage removes the token from the address bar`, `RegisterPage without a token is unchanged`.
- [ ] **Step 2: Run** `cd web && npm test -- src/pages/RegisterPage.test.tsx src/pages/admin/NotificationsPage.test.tsx` — Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npm run lint && npm run typecheck && npm test` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(web): notification routing and registration links from the bot`

### Task 12: Bot setup guide and end-to-end check

**Files:**
- Create: `docs/telegram-bot-setup.md`
- Modify: `docs/deploy-railway.md` (link + "after deploying M2" note), `docs/development.md` (how to test chats locally)

**Interfaces:**
- Consumes: everything above.

- [ ] **Step 1: Write the guide** with the five sections of spec 5.10 (privacy mode off and re-add caveat; `/setjoingroups` enable → add → disable; rights per chat role; how to check; re-adding the bot to an existing journal group after deploying M2). No real chat names.
- [ ] **Step 2: End-to-end run** with the M1 e2e harness (`runWith` + fake Telegram API): feed recorded updates — bot added to a group by the bootstrap admin, a forum message with a topic, a group migration, an unknown phone's contact — then through the API: create label "journal", bind it, set it for applications, submit the application with the token from the bot reply, check the fake API received the group notice (no phone) and, after approve, the private "approved" message. Record the commands and results in the PR description.
- [ ] **Step 3: Run** `make lint && make test` (Go vet/lint/race tests, web lint/typecheck/tests, locale parity) — Expected: PASS.
- [ ] **Step 4: Commit** `docs: Telegram bot setup guide for chats and labels`
