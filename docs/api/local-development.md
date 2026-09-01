# Local Development & Manual Testing (`apps/server/`)

This iteration has no Telegram bot and no browser extension — the API is exercised directly
with `curl`. This is the full setup-to-first-schedule walkthrough.

## 1. Prerequisites

- Go 1.23+
- Docker (for Postgres)
- `openssl` (to generate the encryption key)

## 2. Start Postgres

```bash
cd apps/server
docker compose up -d
docker compose ps   # wait for "healthy"
```

`compose.yaml` maps Postgres to **host port 5435**, not the default 5432 — deliberately, to
avoid clashing with other projects' Postgres containers on the same machine. Adjust if 5435 is
also taken locally.

## 3. Configure environment

```bash
cp .env.example .env
```

Then fill in `SESSION_ENCRYPTION_KEY` (see `docs/architecture/data-storage.md` §3):

```bash
openssl rand -base64 32
```

Paste the output into `.env`. The server refuses to start without a valid 32-byte key.
`INTERNAL_API_TOKEN` defaults to `dev` in `.env.example` — every `/api/v1/*` route except
`/healthz` requires it in the `X-Internal-Token` header.

## 4. Run the server

```bash
set -a; source .env; set +a
go run ./cmd/server
```

Migrations apply automatically on startup (`internal/storage/migrations/`, via `goose`). The
server listens on `HTTP_ADDR` (`:8080` by default).

## 5. Run the test suite

```bash
go test ./...
```

Covers cookie encryption round-trips/tampering (`internal/crypto`), week-parity math across
year boundaries, subject normalization, and the merge engine's matching/discard rules
(`internal/engine`).

## 6. Smoke test — no cookies required

```bash
curl -s localhost:8080/healthz

curl -s -H "X-Internal-Token: dev" 'localhost:8080/api/v1/time/current'

curl -s -H "X-Internal-Token: dev" 'localhost:8080/api/v1/groups?query=ІП-54'

# unlinked user — expect ERR_AUTH_REQUIRED
curl -s -H "X-Internal-Token: dev" 'localhost:8080/api/v1/schedule/today?telegram_id=999'

# missing/invalid token — expect 401
curl -s -o /dev/null -w '%{http_code}\n' 'localhost:8080/api/v1/time/current'
```

## 7. Full flow — requires real `my.kpi.ua` cookies

Log into `my.kpi.ua` in a browser, then copy `PHPSESSID` and `_identity` from DevTools →
Application → Cookies.

```bash
# 1. Capture an HTML fixture (only needed once, to unblock parser development —
#    see docs/schedules/main/data-extraction.md). Requires DEBUG_ROUTES=true.
curl -X POST -H "X-Internal-Token: dev" -H 'Content-Type: application/json' \
  -d '{"cookies":{"PHPSESSID":"<value>","_identity":"<value>"}}' \
  localhost:8080/api/v1/debug/mykpi/dump

# 2. Link the session (probes my.kpi.ua, resolves the group, stores, runs first refresh)
curl -X POST -H "X-Internal-Token: dev" -H 'Content-Type: application/json' \
  -d '{"telegram_id":1,"group_name":"ІП-54","cookies":{"PHPSESSID":"<value>","_identity":"<value>"}}' \
  localhost:8080/api/v1/auth/session

# 3. Fetch schedules
curl -H "X-Internal-Token: dev" 'localhost:8080/api/v1/schedule/today?telegram_id=1'
curl -H "X-Internal-Token: dev" 'localhost:8080/api/v1/schedule/tomorrow?telegram_id=1'
curl -H "X-Internal-Token: dev" 'localhost:8080/api/v1/schedule/week?telegram_id=1&week=1'
curl -H "X-Internal-Token: dev" 'localhost:8080/api/v1/schedule/date?telegram_id=1&date=2026-09-15'

# 4. Force a re-scrape
curl -X POST -H "X-Internal-Token: dev" 'localhost:8080/api/v1/schedule/refresh?telegram_id=1'

# 5. Check status / unlink
curl -H "X-Internal-Token: dev" 'localhost:8080/api/v1/auth/status/1'
curl -X DELETE -H "X-Internal-Token: dev" 'localhost:8080/api/v1/auth/unlink/1'
```

## 8. Expiry path (optional, to see the stale-serve behavior)

After step 2 above has produced a stored schedule, call `auth/session` again with a garbage
`PHPSESSID` for the same `telegram_id`. `schedule/today` should still return the previously
stored lessons, but with `"stale": true` and `"session_status": "expired"` — see
`docs/architecture/data-storage.md` §4.

## 9. Stop everything

```bash
docker compose down          # keeps the named volume (data survives)
docker compose down -v       # also deletes the volume
```
