# Data Storage, Encryption & Refresh Policy

## 1. What Is Persisted, and What Is Not

| Data | Persisted? | Why |
| :--- | :--- | :--- |
| Personal schedule (merged/enriched lessons) | **Yes** — `user_lessons` | It is the point of the product; re-scraping on every request would be slow and hammer `my.kpi.ua`. |
| `my.kpi.ua` session cookies | **Yes, encrypted** — `user_sessions` | Avoids forcing the user to re-authenticate every 1–6 hours; see §3. |
| Group schedule (`api.campus.kpi.ua`) | **No** | Public, cheap to fetch, and would go stale silently. Cached in-memory only (`internal/cache`, TTL 6h), never written to Postgres. |
| Group/lecturer catalogs, lesson-slot times | **No** | Same reasoning; in-memory TTL cache (24h). |

## 2. Schema

```sql
users                (id, telegram_id UNIQUE, group_id, group_name, created_at, updated_at)
user_sessions        (user_id PK/FK, ciphertext, user_agent, status, synced_at, last_checked_at, last_error)
user_schedule_state  (user_id PK/FK, refreshed_at, lesson_count, enrichment_status, last_error)
user_lessons         (id, user_id FK, week, day, slot, start_time, subject, subject_norm,
                       tag, type, lecturer_id, lecturer_name, location_title, location_uri,
                       dates text[], enriched,
                       UNIQUE (user_id, week, day, slot, subject_norm))
```

Migrations live in `apps/server/internal/storage/migrations/` (embedded, applied on startup via `goose`).

`telegram_id` is the external key even though the Telegram bot doesn't exist yet in this
iteration — the API is tested manually with an arbitrary integer, and no schema change will
be needed once the bot is added.

## 3. Cookie Encryption

Cookies are encrypted with **AES-256-GCM** before being written to `user_sessions.ciphertext`:

- Key: `SESSION_ENCRYPTION_KEY` env var, base64-encoded, must decode to exactly 32 bytes. The
  server refuses to start without it.
- Nonce: random 12 bytes, generated per encryption, prepended to the ciphertext.
- AAD (additional authenticated data): the owning user's UUID. This binds a ciphertext to its
  row — copying `ciphertext` into another user's row fails decryption rather than silently
  succeeding.

Implementation: `apps/server/internal/crypto/aesgcm.go` (`Seal`/`Open`), covered by
round-trip and tamper tests in `aesgcm_test.go`.

## 4. Refresh Policy (No Cron)

There is deliberately no background scheduler in this iteration. A schedule read triggers an
inline refresh when:

1. No `user_schedule_state` row exists yet for the user, **or**
2. `refreshed_at` is older than 14 days (one full KPI week-1/week-2 cycle plus margin), **or**
3. the caller passes `force_refresh=true` (`GET /schedule/*`) or calls `POST /schedule/refresh`.

If the refresh fails because `my.kpi.ua` rejects the stored cookies (`ErrSessionExpired`):
- the session row is marked `status = 'expired'`,
- **if a stored schedule already exists, it is served anyway**, with `stale: true` and
  `session_status: "expired"` in the response — never a hard error just because the cookie
  aged out,
- if no stored schedule exists yet, the request fails with `401 ERR_PERSONAL_SESSION_EXPIRED`.

A refresh replaces the user's entire `user_lessons` set inside one transaction (delete + bulk
insert + `user_schedule_state` upsert), so a failed or partial scrape can never leave a mix of
old and new lessons.

## 5. Staleness Guard on `dates[]`

`user_lessons.dates` is a snapshot from the last refresh. Because `api.campus.kpi.ua`'s
`dates[]` can change between refreshes (e.g. a specific-date class gets rescheduled), a
schedule read for a specific day **re-fetches the group schedule (cache-backed, cheap) and
re-derives `dates[]` live** for every enriched lesson before applying the occurrence-date
filter — rather than trusting the stored copy. If the group schedule cannot be fetched, the
stored `dates[]` is used as a fallback and the response is flagged `enrichment_status:
"degraded"`. See `apps/server/internal/engine/merge.go` (`RelookupDates`) and
`apps/server/internal/api/schedule_service.go` (`buildDay`).
