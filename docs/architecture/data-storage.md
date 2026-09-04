# Data Storage & Sync Policy

> **Correction (post-implementation, architecture decision).** The server used to accept a
> student's `my.kpi.ua` session cookies directly and scrape/parse their schedule itself,
> storing the cookies AES-256-GCM-encrypted so it could re-scrape later. **That is no longer
> the plan.** The decision now is: the server should never see or store a student's my.kpi.ua
> credentials at all. Instead, the (not-yet-built) browser extension will authenticate using
> the student's own already-logged-in browser session, extract the schedule client-side, and
> push the already-parsed lesson list to the server. This is safer (no credential storage to
> protect or leak) and moves the fetch/parse cost off the server entirely. The `user_sessions`
> table, `internal/crypto` (AES-256-GCM), and `internal/mykpi` (the my.kpi.ua HTTP
> client/parser) have all been **removed** — see [`docs/schedules/main/data-extraction.md`](../schedules/main/data-extraction.md)
> for what the extension will need to replicate client-side, and
> [`docs/extension/browser-extension-design.md`](../extension/browser-extension-design.md)
> for the updated extension responsibilities. The ingestion endpoint the extension will POST
> to is **not implemented yet** — see §4.

## 1. What Is Persisted, and What Is Not

| Data | Persisted? | Why |
| :--- | :--- | :--- |
| Personal schedule (merged/enriched lessons) | **Yes** — `user_lessons` | It is the point of the product. |
| `my.kpi.ua` session cookies / any credentials | **No, never** | The server no longer authenticates to `my.kpi.ua` at all. The browser extension does that client-side, in the student's own browser, and only ever sends the server an already-parsed schedule. There is nothing to store or protect. |
| Group schedule, group/lecturer catalogs, lesson-slot times, current-time (`api.campus.kpi.ua`) | **Yes** — `campus_cache` | Public and cheap to re-fetch, but deliberately persisted to *disk* (not RAM) so the cache survives the host VM sleeping and waking — see §5. |

## 2. Schema

```sql
users                (id, telegram_id UNIQUE, group_id, group_name, created_at, updated_at)
user_schedule_state  (user_id PK/FK, refreshed_at, lesson_count, enrichment_status, last_error)
user_lessons         (id, user_id FK, date, week, day, slot, start_time, end_time,
                       subject, subject_norm, tag, teacher_raw, location_raw,
                       lecturer_id, lecturer_name, location_title, location_uri, enriched,
                       is_recurring,
                       UNIQUE (user_id, date, start_time, subject_norm))
user_lesson_urls     (id, user_id FK, subject_norm, tag, url, created_at, updated_at,
                       UNIQUE (user_id, subject_norm, tag))
user_url_prompts     (user_id PK/FK, telegram_id UNIQUE, prompt_message_id, subject_norm,
                       tag, subject_name, updated_at)
bot_groups           (id PK, creator_telegram_id, academic_group_id, academic_group_name,
                       faculty, telegram_chat_id UNIQUE, telegram_chat_title, notifications_enabled, created_at, updated_at)
bot_group_admins     (group_id, telegram_id, username, first_name, status, created_at, updated_at,
                       PRIMARY KEY (group_id, telegram_id))
user_group_prompts   (telegram_id PK, prompt_message_id, action, group_id, bind_chat_id,
                       bind_chat_title, updated_at)
campus_cache         (key PK, value /* JSON */, fetched_at)
pairing_codes        (code PK, telegram_id, expires_at)
user_tokens          (token PK, user_id FK, created_at)
issues               (id PK, number UNIQUE, author_telegram_id, author_username,
                       author_first_name, type, title, body, status, status_by,
                       thread_open, created_at, updated_at)
issue_comments       (id PK, issue_id FK, author_role, author_label, body, created_at)
user_issue_drafts    (telegram_id PK, chat_id, prompt_message_id, step, issue_type,
                       title, issue_id, expires_at, updated_at)
```

Migrations live in `apps/server/internal/storage/migrations/` (embedded, applied on startup
via `goose`). `00001_init.sql` establishes core user and schedule storage; `00002_lesson_urls.sql`
introduces `user_lesson_urls` and `user_url_prompts`; `00003_groups.sql` introduces `bot_groups`
(persisting user-configured academic group bindings for Telegram group chats) and `user_group_prompts`
(tracking interactive prompt states for group creation and academic group updates with zero chat pollution);
`00004_group_lesson_urls.sql` introduces `bot_group_lesson_urls`; `00005_notifications.sql` adds notifications;
`00006_group_admins.sql` introduces `bot_group_admins` for multi-admin co-management;
and `00007_issues.sql` introduces `issues`, `issue_comments` and `user_issue_drafts` for the
`/issues` feedback channel (§2.3).


**Engine: SQLite, not PostgreSQL** (`modernc.org/sqlite`, pure Go — no CGO, so it stays
compatible with the distroless final image). This reverses an earlier decision recorded in
[`docs/project-repository.md`](../project-repository.md) §3.1 ("not SQLite — deployment
targets have ephemeral disks"); see §5 below for why. `database/sql` is used directly
(`internal/storage/db.go`), with `SetMaxOpenConns(1)` to serialize all access through
SQLite's single writer rather than juggling `SQLITE_BUSY` retries. IDs are app-generated
UUIDs (`uuid.New()` before insert) rather than a DB-side default, and timestamps are set from
Go (`time.Now().UTC()`) rather than SQL `DEFAULT`, both because SQLite has no
`gen_random_uuid()`/`now()` equivalent that plays well with the DSN's `_time_format=sqlite`
round-tripping.

`telegram_id` is the external key even though the Telegram bot doesn't exist yet in this
iteration — the API is tested manually with an arbitrary integer, and no schema change will
be needed once the bot is added.

`user_lessons`' own history: the original plan modeled it on a week/day/slot recurring
pattern, mirroring `api.campus.kpi.ua`'s shape. Real testing against `my.kpi.ua`'s personal
feed (back when the server still did that fetch itself) showed it actually returns concrete,
already-dated lesson occurrences — see
[`docs/schedules/main/data-extraction.md`](../schedules/main/data-extraction.md). `date` is
therefore the authoritative column and part of the uniqueness key; `week`/`day`/`slot` are
*derived* at merge time (via `engine.WeekAt`/`engine.ISODay` against the Campus API's
current-week anchor, and `engine.slotByTime` against Campus's lesson-slot times) and kept
only for display grouping and for matching against the Campus API's own week-pattern
schedule during enrichment. That part of the schema is unaffected by the credential-storage
decision above and remains correct.

`is_recurring` is a separate, related bit: whether this lesson happens every week of its
`week` parity, or only on specific calendar dates per the matched Campus group lesson's
`dates[]` (defaults `true` when unenriched). It exists specifically so a **stale** stored
schedule keeps rendering correctly without a live Campus fetch: `date` alone is enough for
`/today`/`/date` reads, but the generic `/week` template needs to know which lessons are
safe to show as permanent weekly fixtures and which are one-off sessions that should only
ever appear on their actual occurring dates. See
[`merging-engine.md`](merging-engine.md) §6.

### 2.1 Custom Lesson URLs & Active Prompts (`user_lesson_urls`, `user_url_prompts`)

To let students attach online conference links (Zoom, Google Meet, Teams, etc.) to their classes,
migration `00002_lesson_urls.sql` adds two dedicated tables:

#### Table `user_lesson_urls`
```sql
CREATE TABLE user_lesson_urls (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject_norm TEXT NOT NULL,
    tag          TEXT NOT NULL DEFAULT '',
    url          TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL,
    UNIQUE (user_id, subject_norm, tag)
);

CREATE INDEX idx_user_lesson_urls_user ON user_lesson_urls (user_id);
```

- **Why a separate table instead of a column on `user_lessons`**:
  `storage.ReplaceLessons` runs inside a transaction that deletes all existing `user_lessons` for
  a student (`DELETE FROM user_lessons WHERE user_id = ?`) before inserting the freshly parsed batch.
  Storing URLs directly in `user_lessons` would cause every extension re-sync to either erase all custom
  URLs or require complicated pre-delete extraction and post-insert reconciliation. Storing URLs in
  `user_lesson_urls` completely decouples student customization from schedule ingestion: schedule
  replacements can happen at any time without touching `user_lesson_urls`.
- **Lesson Identity Key (`subject_norm, tag`)**:
  - `subject_norm`: Collapses whitespace, removes non-essential punctuation, and lower-cases the
    course title via `engine.NormalizeSubject`. This shields stored URLs from trivial typography
    changes between scrapes.
  - `tag`: Distinguishes between lectures (`"lec"`), practices/seminars (`"prac"`), and labs (`"lab"`).
    This allows students to assign distinct meeting links to lectures and practices of the same discipline.
- **Online vs. Offline Handling**:
  `model.LocationKind` inspects raw and enriched location strings. Offline classes (e.g. rooms like `"18-402"`)
  are automatically excluded from the `/urls` configuration menu, preventing useless URL prompts for
  on-campus classes.

#### Table `user_url_prompts`
```sql
CREATE TABLE user_url_prompts (
    user_id           TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    telegram_id       INTEGER NOT NULL UNIQUE,
    prompt_message_id INTEGER NOT NULL,
    subject_norm      TEXT NOT NULL,
    tag               TEXT NOT NULL DEFAULT '',
    subject_name      TEXT NOT NULL,
    updated_at        TIMESTAMP NOT NULL
);
```

- **Clean Chat UX via In-Place Mutation**:
  When a student selects a lesson to edit in the bot, the server saves an active prompt row in
  `user_url_prompts` recording the Telegram `prompt_message_id`. When the student replies with a URL:
  1. The bot calls Telegram's `deleteMessage` API to **immediately delete the student's text message**,
     preventing chat pollution when multiple URLs are entered.
  2. The bot edits the original message (`prompt_message_id`) in place to show success or error states.
- **Persistence Across Restarts**:
  Because the prompt state is stored in SQLite (rather than volatile in-memory maps), active prompts survive
  server restarts, deployments, and idle VM sleep cycles without dropping user interactions.

### 2.2 Telegram Bot Groups & Group Lesson URLs (`bot_groups`, `bot_group_lesson_urls`, `user_group_prompts`)

To enable the bot to serve schedules inside Telegram group chats, migrations `00003_groups.sql` and `00004_group_lesson_urls.sql` introduce:

#### Table `bot_groups`
Stores bot group bindings configured by users in private messages:
- `id TEXT PRIMARY KEY` (UUID)
- `creator_telegram_id INTEGER NOT NULL`
- `academic_group_id INTEGER NOT NULL` (Campus API group ID)
- `academic_group_name TEXT NOT NULL` (e.g. "ІП-21")
- `faculty TEXT NOT NULL DEFAULT ''`
- `telegram_chat_id INTEGER UNIQUE` (linked group chat)
- `telegram_chat_title TEXT NOT NULL DEFAULT ''`

#### Table `bot_group_lesson_urls`
Allows group administrators to configure online conference links (Zoom, Google Meet, Microsoft Teams) for group lessons from the DM settings menu:
- `id TEXT PRIMARY KEY` (UUID)
- `group_id TEXT NOT NULL REFERENCES bot_groups(id) ON DELETE CASCADE`
- `subject_norm TEXT NOT NULL`
- `tag TEXT NOT NULL DEFAULT ''`
- `url TEXT NOT NULL`
- `UNIQUE (group_id, subject_norm, tag)`

#### Table `user_group_prompts`
Tracks multi-step input prompts for creating groups, editing academic group names, or setting group lesson URLs (`action`: `"create"`, `"edit_academic"`, `"set_url"`).
- Records `telegram_id`, `prompt_message_id`, `action`, `group_id`, `subject_norm`, `tag`, `subject_name`, `bind_chat_id`, and `bind_chat_title`.

#### Table `bot_group_admins`
Tracks co-administrators invited by the group creator (`00006_group_admins.sql`):
- `group_id TEXT NOT NULL REFERENCES bot_groups(id) ON DELETE CASCADE`
- `telegram_id INTEGER NOT NULL`
- `username TEXT NOT NULL DEFAULT ''`
- `first_name TEXT NOT NULL DEFAULT ''`
- `status TEXT NOT NULL DEFAULT 'invited'` (`invited` when invited by creator, `accepted` once confirmed in chat)
- `created_at TIMESTAMP NOT NULL`, `updated_at TIMESTAMP NOT NULL`
- `PRIMARY KEY (group_id, telegram_id)`
- Index: `idx_bot_group_admins_user (telegram_id, status)`

**Ownership transfer on deletion**: when the creator leaves or deletes the group, ownership is automatically transferred to the earliest accepted administrator in `bot_group_admins`. If no accepted administrators remain, the group configuration, associated URLs, prompts, and admin records are deleted in a cascade.


### 2.3 User-Filed Issues (`issues`, `issue_comments`, `user_issue_drafts`)

Backing store for the bot's `/issues` command and the dashboard's issue queue. Flow and screen
behaviour are documented in [`docs/bot/issues.md`](../bot/issues.md); the endpoints admins reach
them through are in [`docs/api/admin-endpoints.md`](../api/admin-endpoints.md).

#### Table `issues`
One row per bug report or feature request (`00007_issues.sql`):
- `id TEXT PRIMARY KEY` — app-generated UUID, used everywhere internally.
- `number INTEGER NOT NULL UNIQUE` — the public, human-facing `#12`. A single **global** sequence
  (not per user), assigned inside the insert transaction as `COALESCE(MAX(number), 0) + 1`. This is
  safe rather than racy because the pool is capped at one connection (`SetMaxOpenConns(1)`), and the
  `UNIQUE` constraint is the backstop. Deliberately not `AUTOINCREMENT`: every other table here uses
  a UUID primary key, and the public number is a separate concern from row identity.
- `author_telegram_id INTEGER NOT NULL` — the reporter. Stored directly rather than as a
  `users(id)` foreign key, matching `bot_groups.creator_telegram_id`: filing an issue does not
  require a linked account, so there may be no `users` row to point at.
- `author_username TEXT`, `author_first_name TEXT` — captured at creation so the dashboard can
  identify the reporter without a second lookup.
- `type TEXT NOT NULL CHECK (type IN ('feature','bug','other'))` — fixed at creation.
- `title TEXT NOT NULL`, `body TEXT NOT NULL` — capped by the bot at 120 / 3000 runes.
- `status TEXT NOT NULL DEFAULT 'on_review' CHECK (status IN ('on_review','ready','in_development','implemented','cancelled'))`
  — changed only by admins, from the dashboard.
- `status_by TEXT NOT NULL DEFAULT ''` — email of the admin who last changed the status, taken
  from the `X-Admin-Email` header the dashboard already forwards. This is the feature's audit
  trail: the shared telemetry pipeline anonymises identifiers and cannot carry it.
- `thread_open BOOLEAN NOT NULL DEFAULT 0` — flips true on the first admin comment. Threads are
  admin-initiated; users see the discussion button only once it is set.
- `created_at TIMESTAMP NOT NULL`, `updated_at TIMESTAMP NOT NULL` — `updated_at` also moves on
  thread activity, so the dashboard can sort by recency.
- Indexes: `idx_issues_author (author_telegram_id, created_at)`, `idx_issues_status (status, created_at)`.

#### Table `issue_comments`
The discussion transcript between an admin and the reporter:
- `id TEXT PRIMARY KEY`, `issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE`
- `author_role TEXT NOT NULL CHECK (author_role IN ('user','admin'))`
- `author_label TEXT NOT NULL DEFAULT ''` — the admin's email, or the reporter's `@username`.
- `body TEXT NOT NULL`, `created_at TIMESTAMP NOT NULL`
- Index: `idx_issue_comments_issue (issue_id, created_at)` — threads are always read oldest-first.

#### Table `user_issue_drafts`
In-flight `/issues` wizard state — one row per user, replaced when a new flow starts:
- `telegram_id INTEGER PRIMARY KEY`, `chat_id INTEGER NOT NULL`, `prompt_message_id INTEGER NOT NULL`
  — which bot message the wizard keeps editing.
- `step TEXT NOT NULL` (`title` | `body` | `reply`), `issue_type TEXT`, `title TEXT`,
  `issue_id TEXT` (set only for `reply` drafts).
- `expires_at TIMESTAMP NOT NULL` — 10 minutes out, per `storage.IssueDraftTTL`.
  Index: `idx_user_issue_drafts_expiry (expires_at)`.

**Why this is persisted and not held in memory.** The request was for ten minutes of in-memory
state, but the server sleeps after 15 minutes idle on Fly.io
([fly-scale-to-zero.md](fly-scale-to-zero.md)). A process-local map would lose drafts whenever the
machine slept mid-flow, and — worse — lose the `prompt_message_id` needed to clean up the bot's own
message afterwards. A row costs nothing and survives sleep, restarts and deploys.

**Expiry is two-sided**, following the `pairing_codes` idiom (`expires_at` + purge-on-read):
`GetIssueDraft` deletes and reports an expired row exactly once (`ErrIssueDraftExpired`), so the
next interaction can tell the user the flow was interrupted; and a per-minute sweep
(`(*bot.Bot).SweepExpiredIssueDrafts`, driven by the cron tick that already survives
scale-to-zero) deletes the abandoned wizard message before clearing the row — something lazy
expiry cannot do, since an abandoned draft is never read again.

## 3. No Credential Storage, No Encryption

There used to be a §3 here describing AES-256-GCM cookie encryption
(`internal/crypto/aesgcm.go`, `SESSION_ENCRYPTION_KEY`). It's gone, along with the code —
the server has no secret to encrypt any more. If a future feature genuinely needs
at-rest encryption again, re-add it deliberately rather than reviving this section.

## 4. Sync Policy (No Cron, No Server-Side Refresh)

There is no background scheduler, and — unlike the earlier design — **the server cannot
trigger a refresh of its own**. It has no credentials to fetch with. The only way schedule
data changes is a push from the browser extension.

- **Ingestion endpoint: not yet implemented.** The extension isn't built yet, so neither is
  its server-side receiving endpoint. When it is, expect something like `POST
  /api/v1/schedule/sync` accepting `{telegram_id, group_name?, lessons: [...]}` (the shape
  in `model.ParsedLesson`), which resolves/creates the user, merges the submitted lessons
  against the Campus group schedule via `engine.Merge` (unchanged — see
  [`merging-engine.md`](merging-engine.md)), and replaces the user's `user_lessons` set in
  one transaction (`storage.ReplaceLessons`, unchanged).
- **Reads are passive.** `GET /schedule/*` only ever reads what's already stored
  (`Service.buildDay`/`buildWeek`) — no inline fetch, no `force_refresh` parameter (there is
  nothing left to force). If no schedule has ever been pushed for a user, reads fail with
  `401 ERR_AUTH_REQUIRED` telling the caller to sync the extension first.
- **`stale` is informational only.** A response is flagged `stale: true` once
  `user_schedule_state.refreshed_at` is older than 14 days (one full KPI week-1/week-2 cycle
  plus margin) — meaning "the extension hasn't pushed an update in a while," not "your
  session expired" (there is no session to expire). The old `session_status` field
  (`active`/`expired`) is gone entirely along with the session concept.

## 5. Disk-Backed Campus API Cache (SQLite, Not In-Memory)

The hosting plan for this server is a VM that **sleeps when idle** (backed by a persistent
disk) to save cost. An in-memory cache (the old `internal/cache.TTL[V]`) is wiped every time
the VM sleeps and its process is torn down, so every wake would cause a cold-start burst of
re-fetches against `api.campus.kpi.ua` (group catalog, per-group schedules, lesson slots,
current time) before the server could serve a single request cheaply. Moving that cache onto
disk — SQLite lives in the same persistent-disk-backed file as `user_lessons` — means a value
fetched before the VM slept is still warm on wake.

- **Table**: `campus_cache (key TEXT PK, value TEXT /* JSON */, fetched_at TIMESTAMP)`.
- **API**: `storage.DB.CacheGet(ctx, key, maxAge, &out) (bool, error)` /
  `storage.DB.CacheSet(ctx, key, value) error` (`internal/storage/campuscache.go`) —
  JSON-encodes the value, and treats an entry as a miss once `time.Since(fetched_at) >
  maxAge`, mirroring the old TTL semantics exactly.
- **Callers**: `internal/campus/client.go`'s `cachedJSON` helper wraps every Campus API call
  with the same TTLs the in-memory cache used — `time/current` 1 min, lesson slots 24h, group
  catalog 24h, per-group schedule 6h (keyed by group ID).
- **What did *not* change**: the per-IP rate limiter (`internal/api/middleware.go`,
  `github.com/go-chi/httprate`) stays in-memory on purpose. Its window is 1 minute, and any
  VM sleep cycle is guaranteed to exceed that — losing the in-memory counter on sleep is a
  non-issue, so persisting it would only add write load for no benefit. See
  [`error-handling-resilience.md`](error-handling-resilience.md) §5.
- **Deployment**: the SQLite file lives at `DATABASE_PATH` (default `/data/kpi.db` in the
  container), which is mounted on a Fly Volume backed by NVMe storage — see the
  `Dockerfile`'s `VOLUME ["/data"]`, `fly.toml`'s `[mounts]`, and
  [`fly-scale-to-zero.md`](fly-scale-to-zero.md) for the complete 15-minute idle scale-to-zero architecture.
