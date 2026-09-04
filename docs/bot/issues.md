# Issues (`/issues`) — User-Filed Bug Reports & Feature Requests

`/issues` is a lightweight, GitHub-issues-shaped feedback channel inside the bot. A user files a bug report or feature request through a guided wizard, gets a numbered issue back (`#12`), and can watch it move through triage. Admins triage the queue from the [Admin Web Dashboard](../architecture/admin-dashboard.md).

> **Language note.** Every `/issues` screen is written in **Ukrainian**, like the rest of the bot. Keep new strings in this feature Ukrainian too. Ukrainian inflects, so validation wording lives in `issueField` values (`issueFieldTitle`, `issueFieldBody`, `issueFieldReply`) rather than one shared template, and message counts go through `pluralUA` (1 / 2–4 / 5+ forms).

Implementation: [`internal/bot/issues.go`](../../apps/server/internal/bot/issues.go) (handlers), [`internal/bot/issues_render.go`](../../apps/server/internal/bot/issues_render.go) (screens and keyboards), [`internal/storage/issues.go`](../../apps/server/internal/storage/issues.go) (persistence), migration `00007_issues.sql`.

---

## 1. Scope & Access

- **DM only**, enforced on all three entry points rather than by the command menu alone:
  - `/issues` is registered under `BotCommandScopeAllPrivateChats` only. Typed in a group anyway, `cmdIssues` replies with the standard DM-only notice and does nothing else.
  - `onIssues` refuses every `iss:` callback that arrives from a group chat with the same notice as an alert. No issue screen can legitimately exist there, so such a tap was forwarded or crafted.
  - `onTextMessage` does not even look up a wizard draft for a message sent in a group. The draft is keyed by `telegram_id`, so without this a user with an open wizard would have had their group messages deleted and filed as issue text. The DM path additionally checks `draft.chat_id` against the chat the message came from.
- **No linked account required.** Unlike `/today` or `/urls`, filing an issue does not need a paired browser extension — anyone who can talk to the bot can report a problem.
- **Private by author.** A user only ever sees issues they filed. `iss:view:` re-checks `author_telegram_id` against the caller before rendering, so a leaked or guessed callback payload cannot open someone else's issue.

## 2. Statuses

Only admins change status, from the dashboard. Users read it.

| Value | Label shown to the user | Meaning |
| :--- | :--- | :--- |
| `on_review` | 🕓 На розгляді | Default on creation. Nobody has picked it up yet. |
| `ready` | 📌 Готово до розробки | Accepted, queued for a future release. |
| `in_development` | 🔨 У розробці | Being worked on now. |
| `implemented` | ✅ Реалізовано | Shipped. |
| `duplicate` | 🔁 Дублікат | Already filed by someone else. |
| `rejected` | ⛔ Відхилено | Considered and declined. |
| `cancelled` | 🚫 Скасовано | Won't be done. |

Types are fixed at creation and never change: `feature` (💡 Пропозиція), `bug` (🐞 Помилка), `other` (📝 Інше).

A status change can carry an **optional note** from the admin — a single message explaining the decision, typically alongside `rejected` or `duplicate`. It arrives with the status DM and stays on the issue screen under *Коментар команди*, so it can be re-read later. A note does **not** open a discussion; it is one-way. Writing a change with no note clears the previous one, so a stale explanation never outlives the status it explained.

## 3. Screen Flow

The whole wizard lives in **one bot message**, edited in place. Every answer the user types is deleted immediately, so a completed flow leaves exactly one message in the chat — the same zero-pollution policy as the `/urls` and `/group` prompts (see [telegram-bot-design.md §5](telegram-bot-design.md#5-inline-navigation--message-mutation)).

```
/issues  ─►  📮 Звернення
             ├─ 📋 Мої звернення ──► list (5 per page) ──► issue view ──┬─ 💬 Обговорення
             │                                                          └─ 🗑 Видалити ──► confirm
             └─ ➕ Нове звернення ──► type ──► title ──► description ──► ✅ created
```

| Step | Screen | Buttons |
| :--- | :--- | :--- |
| 0 | Root menu | `📋 Мої звернення`, `➕ Нове звернення` |
| 1 | Type picker | `💡 Пропозиція`, `🐞 Помилка`, `📝 Інше`, **`✖️ Скасувати`** |
| 2 | Title prompt (Крок 1 з 2) | `◀️ Назад` (→ type picker), `✖️ Скасувати` |
| 3 | Description prompt (Крок 2 з 2) | `◀️ Назад` (→ title), `✖️ Скасувати` |
| 4 | Created | `📋 Мої звернення` |

**Крок 1 has no Back on purpose** — it is the first step, so going back *is* cancelling, and the picker offers Cancel instead. **Step 4 has neither** — the issue already exists and there is nothing left to abandon.

`✖️ Скасувати` discards the draft and **deletes the bot's own message**, leaving no trace of the abandoned flow.

### Success message

The created screen must carry the issue headline. The bot sends `ParseMode: "HTML"`, so the required `` **`#N` title** `` renders as `<b><code>#N</code> title</b>`:

```text
✅ Звернення створено

#12 Експорт у календар
💡 Пропозиція · 🕓 На розгляді

Дякуємо! Команда невдовзі його розгляне.
Перевірити статус можна будь-коли командою /issues.
```

### Input limits

Title ≤ 120 characters, description ≤ 3000 — both counted in **runes**, so Cyrillic input is not penalised. A rejected answer re-renders the same prompt with an `❌` error line above it rather than posting a new message, mirroring `formatURLPrompt`.

### Deleting an issue

The issue screen carries a `🗑 Видалити` button. It never deletes on the first tap: it opens a confirmation screen (`🗑 Так, видалити` / `◀️ Залишити`) that spells out that the issue and its whole discussion go permanently, for the reporter and the team alike. Confirming returns the user to their list.

Deletion is scoped by authorship the same way viewing is — `authoredIssue` re-checks `author_telegram_id` before both the confirmation and the delete itself, so a guessed callback cannot destroy someone else's issue. Admins can delete from the dashboard too; see [admin-endpoints.md §2.10](../api/admin-endpoints.md).

## 4. Draft State & the 10-Minute TTL

An in-flight wizard is a row in `user_issue_drafts`, keyed by `telegram_id` — one draft per user, so starting a new flow replaces the old one. Starting a wizard also clears any pending `/urls` or `/group` prompt, since free text now belongs to the wizard.

**Why SQLite and not an in-memory map:** the server runs on Fly.io with a 15-minute idle scale-to-zero shutdown ([fly-scale-to-zero.md](../architecture/fly-scale-to-zero.md)). Process memory does not survive the machine sleeping, so an in-memory draft would vanish mid-flow — and, worse, the bot would no longer know which message to clean up. The row survives sleep, restarts and deploys.

Expiry has two halves:

1. **Lazy, on read.** `GetIssueDraft` deletes any row past `expires_at` and returns `ErrIssueDraftExpired` — exactly once, since the row is consumed. It hands the expired draft back alongside the error so the caller still knows which wizard message to remove; the sweeper will never see that row again. The bot then shows the root menu with the interrupted banner: *"Це зайняло забагато часу. Чернетку втрачено через 10 хвилин — нічого не збережено."*
2. **Sweep, on a heartbeat.** `(*bot.Bot).SweepExpiredIssueDrafts` lists expired rows, deletes the wizard message each one left behind, then clears the row. Lazy expiry alone cannot do this: an abandoned draft is by definition never touched again. The sweep runs from the per-minute cron tick (`/api/v1/cron/lesson-alerts`, the only heartbeat that survives scale-to-zero) and from the in-process ticker used when idle shutdown is disabled. Message deletion is best-effort — Telegram refuses to delete messages older than 48 hours — but the draft is dropped either way.

`storage.IssueDraftTTL` (10 minutes) is a single constant; lower it temporarily to exercise the sweep by hand.

## 5. Callback Namespace

All buttons share the `iss:` prefix and one dispatcher handler (`onIssues`), following the one-prefix-per-screen convention in [`callback.go`](../../apps/server/internal/bot/callback.go).

| Callback data | Action |
| :--- | :--- |
| `iss:menu` | Root menu (clears any draft) |
| `iss:new` | Type picker (clears any draft) |
| `iss:type:<feature\|bug\|other>` | Start a draft, prompt for the title |
| `iss:back:title` | Rewind from description to title |
| `iss:cancel` | Discard the draft and delete the bot's message |
| `iss:list:<page>` | The caller's issues, 5 per page |
| `iss:view:<page>:<uuid>` | One issue; the page rides along so `◀️ Назад` returns where the user came from |
| `iss:thr:<uuid>` | Discussion thread (see §6) |
| `iss:reply:<uuid>` | Prompt for a thread reply (see §6) |
| `iss:del:<page>:<uuid>` | Delete confirmation for one of the caller's own issues |
| `iss:delok:<page>:<uuid>` | Confirmed delete, then back to the list page |

Payloads stay well inside Telegram's 64-byte `callback_data` limit (`iss:view:0:` + a 36-character UUID = 47 bytes).

## 6. Discussion Threads

Threads are **admin-initiated**, per the feature's spec. `issues.thread_state` starts at `none` and moves to `open` the first time an admin comments from the dashboard — in the *same transaction* as the comment insert (`AddIssueComment`), so a stored comment can never leave the reporter with a "Відкрити обговорення" DM whose button leads to a thread the bot still thinks is unstarted. Only then does the user see a `💬 Обговорення (N)` button on the issue view. A user cannot open a thread unilaterally — `iss:thr:` silently no-ops while the state is `none`, so a guessed callback cannot conjure one either.

An admin can also **close** a thread, and reopen it later:

| `thread_state` | The reporter sees | Can reply? |
| :--- | :--- | :---: |
| `none` | No discussion button at all | — |
| `open` | `💬 Обговорення (N)` | ✅ |
| `closed` | `🔒 Обговорення (N)`, transcript prefaced with why writing stopped | ❌ |

Closing is deliberately not deletion: the history stays fully readable, only the ability to add to it goes away.

| Screen | Buttons |
| :--- | :--- |
| Thread, open (`iss:thr:<uuid>`) | `✍️ Відповісти`, `🔄 Оновити`, `◀️ Назад` (→ issue view) |
| Thread, closed (`iss:thr:<uuid>`) | `🔄 Оновити`, `◀️ Назад` — **no Reply** |
| Reply prompt (`iss:reply:<uuid>`) | `◀️ Назад` (→ thread), `✖️ Скасувати` |

The Reply gate is enforced in three places, not just by hiding the button: `iss:reply:` no-ops unless the state is `open`, and `handleIssueReplyInput` re-checks it before saving — a thread closed between the prompt and the answer drops the reply and shows the closed transcript instead.

The thread renders the history oldest-first, each message in a `<blockquote>`, attributed `👤 Ви` or `🛠 Команда` — the admin's email is never shown to the user. Replies obey the same 3000-rune limit as issue bodies (`model.IssueCommentMaxLen`).

### Fitting a Telegram message

Telegram rejects a message whose **parsed** text (markup and entity escapes do not count) exceeds 4096 characters, and a rejected edit leaves the tapped button spinning forever. A transcript grows without bound, and an issue body and a status note are 3000 runes each, so two screens are rendered against a budget (`issueScreenBudget`, measured with `renderedLen`):

- **Issue view** splits what the header leaves of the budget between the body and the team's note, giving the note at most half. Both are elided with `…` when they do not fit.
- **Discussion thread** fills newest-first and drops the oldest messages, prefacing the transcript with *"… приховано N попередніх повідомлень. Повне обговорення — у панелі команди."* A message with less than `minCommentPreview` characters of room is dropped rather than shown as a stub. Short threads render whole, with no notice.

`applyScreen` also answers the callback query on an edit failure, so any screen that still manages to be unsendable stops the spinner instead of hanging the button.

`✍️ Відповісти` writes a draft with `step = "reply"` and `issue_id` set, under the same 10-minute TTL as the creation wizard (§4), and reuses the identical mechanics: the user's message is deleted, the bot's single message is edited in place, and the thread re-renders with the new comment appended. `✖️ Скасувати` deletes the bot's message, exactly as in the wizard.

### Notification DMs

All three notifications are sent by [`issues_notify.go`](../../apps/server/internal/bot/issues_notify.go), which implements the `api.IssueNotifier` interface. The interface is declared in `internal/api` and satisfied by `*bot.Bot` because `internal/bot` already imports `internal/api` — wiring it the other way round would be an import cycle. It is injected in `cmd/server/main.go` (`svc.SetIssueNotifier(tgBot)`) and left nil when the bot is disabled, in which case the notify helpers are no-ops.

Delivery is **best-effort**: a Telegram failure is logged and the admin's HTTP request still succeeds, because the comment or status change is already committed. A blocked bot or a deleted chat must not break the dashboard.

| Trigger | DM |
| :--- | :--- |
| Admin posts a comment | *💬 Нова відповідь у вашому зверненні*, the headline, and the quoted message; button `💬 Відкрити обговорення` → `iss:thr:<uuid>` |
| Admin changes the status | *🔄 Статус звернення змінився*, the headline, and `🕓 На розгляді → ⛔ Відхилено` — plus the optional note under *Коментар команди*; button `📄 Відкрити звернення` → `iss:view:0:<uuid>` |
| Admin closes or reopens the thread | *🔒 Обговорення закрито* / *💬 Обговорення відновлено*, the headline, and what it means for replying; button `💬 Відкрити обговорення` |

Re-applying a status, or a thread state, an issue already has sends nothing — the endpoints short-circuit before notifying (see [admin-endpoints.md §2.7 and §2.9](../api/admin-endpoints.md)).

Deletion is the exception: it sends **no** DM at all. An issue the reporter can no longer open is not something a message usefully explains, and admin-side deletion is normally spam cleanup.

Both sides see the same transcript: the user in the bot, admins on the dashboard's issue page. See [admin-endpoints.md](../api/admin-endpoints.md) for the endpoints behind it.

## 7. Storage

Three tables, added by `00007_issues.sql` and reshaped by `00008_issue_thread_state.sql`; column-level detail lives in [data-storage.md §2.3](../architecture/data-storage.md).

- `issues` — one row per report. `number` is the public `#N`: a single global sequence assigned inside the insert transaction as `MAX(number) + 1`, safe because the SQLite pool is capped at one connection, with a `UNIQUE` constraint as backstop. `00008_issue_thread_state.sql` later widened the `status` CHECK (`duplicate`, `rejected`), replaced `thread_open` with `thread_state`, and added `status_note`.
- `issue_comments` — the discussion transcript, `ON DELETE CASCADE` from `issues`, so deleting an issue takes its thread with it.
- `user_issue_drafts` — in-flight wizard state (§4).
