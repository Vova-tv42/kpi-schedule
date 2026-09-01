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
| Group schedule (`api.campus.kpi.ua`) | **No** | Public, cheap to fetch, and would go stale silently. Cached in-memory only (`internal/cache`, TTL 6h), never written to Postgres. |
| Group/lecturer catalogs, lesson-slot times | **No** | Same reasoning; in-memory TTL cache (24h). |

## 2. Schema

```sql
users                (id, telegram_id UNIQUE, group_id, group_name, created_at, updated_at)
user_schedule_state  (user_id PK/FK, refreshed_at, lesson_count, enrichment_status, last_error)
user_lessons         (id, user_id FK, date, week, day, slot, start_time, end_time,
                       subject, subject_norm, tag, teacher_raw, location_raw,
                       lecturer_id, lecturer_name, location_title, location_uri, enriched,
                       UNIQUE (user_id, date, start_time, subject_norm))
```

Migrations live in `apps/server/internal/storage/migrations/` (embedded, applied on startup
via `goose`). `00001_init.sql` was edited in place (again) to drop `user_sessions` rather
than layered with a new migration file — nothing is deployed yet.

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
