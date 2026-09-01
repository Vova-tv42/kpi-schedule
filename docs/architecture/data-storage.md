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
campus_cache         (key PK, value /* JSON */, fetched_at)
```

Migrations live in `apps/server/internal/storage/migrations/` (embedded, applied on startup
via `goose`). `00001_init.sql` was edited in place (again) to drop `user_sessions` rather
than layered with a new migration file — nothing is deployed yet.

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
  container), which must be a mounted volume backed by the host's persistent disk — see the
  `Dockerfile`'s `VOLUME ["/data"]` and `docs/project-repository.md` §4.2.
