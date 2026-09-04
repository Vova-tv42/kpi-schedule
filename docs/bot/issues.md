# Issues (`/issues`) — User-Filed Bug Reports & Feature Requests

`/issues` is a lightweight, GitHub-issues-shaped feedback channel inside the bot. A user files a bug report or feature request through a guided wizard, gets a numbered issue back (`#12`), and can watch it move through triage. Admins triage the queue from the [Admin Web Dashboard](../architecture/admin-dashboard.md).

> **Language note.** Every `/issues` screen is written in **English**, deliberately, at the product owner's request. It is the only English surface in an otherwise Ukrainian bot. Keep new strings in this feature English, and everything outside it Ukrainian.

Implementation: [`internal/bot/issues.go`](../../apps/server/internal/bot/issues.go) (handlers), [`internal/bot/issues_render.go`](../../apps/server/internal/bot/issues_render.go) (screens and keyboards), [`internal/storage/issues.go`](../../apps/server/internal/storage/issues.go) (persistence), migration `00007_issues.sql`.

---

## 1. Scope & Access

- **DM only.** `/issues` is registered under `BotCommandScopeAllPrivateChats` only. Invoked in a group it replies with the standard DM-only notice and does nothing else.
- **No linked account required.** Unlike `/today` or `/urls`, filing an issue does not need a paired browser extension — anyone who can talk to the bot can report a problem.
- **Private by author.** A user only ever sees issues they filed. `iss:view:` re-checks `author_telegram_id` against the caller before rendering, so a leaked or guessed callback payload cannot open someone else's issue.

## 2. Statuses

Only admins change status, from the dashboard. Users read it.

| Value | Label shown to the user | Meaning |
| :--- | :--- | :--- |
| `on_review` | 🕓 On review | Default on creation. Nobody has picked it up yet. |
| `ready` | 📌 Ready for development | Accepted, queued for a future release. |
| `in_development` | 🔨 In development | Being worked on now. |
| `implemented` | ✅ Implemented | Shipped. |
| `cancelled` | 🚫 Cancelled | Won't be done. |

Types are fixed at creation and never change: `feature` (💡 Feature request), `bug` (🐞 Bug fix), `other` (📝 Other).

## 3. Screen Flow

The whole wizard lives in **one bot message**, edited in place. Every answer the user types is deleted immediately, so a completed flow leaves exactly one message in the chat — the same zero-pollution policy as the `/urls` and `/group` prompts (see [telegram-bot-design.md §5](telegram-bot-design.md#5-inline-navigation--message-mutation)).

```
/issues  ─►  📮 Issues
             ├─ 📋 My issues ──► list (5 per page) ──► issue view ──► 💬 Discussion
             └─ ➕ New issue ──► type ──► title ──► description ──► ✅ created
```

| Step | Screen | Buttons |
| :--- | :--- | :--- |
| 0 | Root menu | `📋 My issues`, `➕ New issue` |
| 1 | Type picker | `💡 Feature request`, `🐞 Bug fix`, `📝 Other`, **`✖️ Cancel`** |
| 2 | Title prompt (step 1 of 2) | `◀️ Back` (→ type picker), `✖️ Cancel` |
| 3 | Description prompt (step 2 of 2) | `◀️ Back` (→ title), `✖️ Cancel` |
| 4 | Created | `📋 My issues` |

**Step 1 has no Back on purpose** — it is the first step, so going back *is* cancelling, and the picker offers Cancel instead. **Step 4 has neither** — the issue already exists and there is nothing left to abandon.

`✖️ Cancel` discards the draft and **deletes the bot's own message**, leaving no trace of the abandoned flow.

### Success message

The created screen must carry the issue headline. The bot sends `ParseMode: "HTML"`, so the required `` **`#N` title** `` renders as `<b><code>#N</code> title</b>`:

```text
✅ Issue created

#12 Add calendar export
💡 Feature request · 🕓 On review

Thanks! The team will review it soon.
You can track its status any time with /issues.
```

### Input limits

Title ≤ 120 characters, description ≤ 3000 — both counted in **runes**, so Cyrillic input is not penalised. A rejected answer re-renders the same prompt with an `❌` error line above it rather than posting a new message, mirroring `formatURLPrompt`.

## 4. Draft State & the 10-Minute TTL

An in-flight wizard is a row in `user_issue_drafts`, keyed by `telegram_id` — one draft per user, so starting a new flow replaces the old one. Starting a wizard also clears any pending `/urls` or `/group` prompt, since free text now belongs to the wizard.

**Why SQLite and not an in-memory map:** the server runs on Fly.io with a 15-minute idle scale-to-zero shutdown ([fly-scale-to-zero.md](../architecture/fly-scale-to-zero.md)). Process memory does not survive the machine sleeping, so an in-memory draft would vanish mid-flow — and, worse, the bot would no longer know which message to clean up. The row survives sleep, restarts and deploys.

Expiry has two halves:

1. **Lazy, on read.** `GetIssueDraft` deletes any row past `expires_at` and returns `ErrIssueDraftExpired` — exactly once, since the row is consumed. It hands the expired draft back alongside the error so the caller still knows which wizard message to remove; the sweeper will never see that row again. The bot then shows the root menu with the interrupted banner: *"That took too long. The draft expired after 10 minutes and was discarded — nothing was saved."*
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
| `iss:view:<page>:<uuid>` | One issue; the page rides along so `◀️ Back` returns where the user came from |
| `iss:thr:<uuid>` | Discussion thread (see §6) |
| `iss:reply:<uuid>` | Prompt for a thread reply (see §6) |

Payloads stay well inside Telegram's 64-byte `callback_data` limit (`iss:view:0:` + a 36-character UUID = 47 bytes).

## 6. Discussion Threads

Threads are **admin-initiated**, per the feature's spec. `issues.thread_open` starts false and flips true the first time an admin comments from the dashboard; only then does the user see a `💬 Discussion (N)` button on the issue view. A user cannot open a thread unilaterally — `iss:thr:` silently no-ops while `thread_open` is false, so a guessed callback cannot conjure one either.

| Screen | Buttons |
| :--- | :--- |
| Thread (`iss:thr:<uuid>`) | `✍️ Reply`, `🔄 Refresh`, `◀️ Back` (→ issue view) |
| Reply prompt (`iss:reply:<uuid>`) | `◀️ Back` (→ thread), `✖️ Cancel` |

The thread renders the full history oldest-first, each message in a `<blockquote>`, attributed `👤 You` or `🛠 Team` — the admin's email is never shown to the user. Replies obey the same 3000-rune limit as issue bodies (`model.IssueCommentMaxLen`).

`✍️ Reply` writes a draft with `step = "reply"` and `issue_id` set, under the same 10-minute TTL as the creation wizard (§4), and reuses the identical mechanics: the user's message is deleted, the bot's single message is edited in place, and the thread re-renders with the new comment appended. `✖️ Cancel` deletes the bot's message, exactly as in the wizard.

### Notification DMs

Both notifications are sent by [`issues_notify.go`](../../apps/server/internal/bot/issues_notify.go), which implements the `api.IssueNotifier` interface. The interface is declared in `internal/api` and satisfied by `*bot.Bot` because `internal/bot` already imports `internal/api` — wiring it the other way round would be an import cycle. It is injected in `cmd/server/main.go` (`svc.SetIssueNotifier(tgBot)`) and left nil when the bot is disabled, in which case the notify helpers are no-ops.

Delivery is **best-effort**: a Telegram failure is logged and the admin's HTTP request still succeeds, because the comment or status change is already committed. A blocked bot or a deleted chat must not break the dashboard.

| Trigger | DM |
| :--- | :--- |
| Admin posts a comment | *💬 New reply on your issue*, the headline, and the quoted message; button `💬 Open discussion` → `iss:thr:<uuid>` |
| Admin changes the status | *🔄 Issue status changed*, the headline, and `🕓 On review → 🔨 In development`; button `📄 Open issue` → `iss:view:0:<uuid>` |

Re-applying a status an issue already has sends nothing — the endpoint short-circuits before notifying (see [admin-endpoints.md §2.7](../api/admin-endpoints.md)).

Both sides see the same transcript: the user in the bot, admins on the dashboard's issue page. See [admin-endpoints.md](../api/admin-endpoints.md) for the endpoints behind it.

## 7. Storage

Three tables, all added by `00007_issues.sql`; column-level detail lives in [data-storage.md §2.3](../architecture/data-storage.md).

- `issues` — one row per report. `number` is the public `#N`: a single global sequence assigned inside the insert transaction as `MAX(number) + 1`, safe because the SQLite pool is capped at one connection, with a `UNIQUE` constraint as backstop.
- `issue_comments` — the discussion transcript, `ON DELETE CASCADE` from `issues`.
- `user_issue_drafts` — in-flight wizard state (§4).
