# Local Development & Manual Testing (`apps/server/`)

> **Correction (post-implementation, architecture decision).** The server no longer accepts
> `my.kpi.ua` cookies, so the old "capture a fixture / link a session / force a re-scrape /
> see the expiry path" curl walkthrough below no longer applies — see
> [`docs/architecture/data-storage.md`](../architecture/data-storage.md). Until the browser
> extension and its schedule-sync ingestion endpoint are built, **there is no way to get a
> real schedule into the system via curl.** §7 below documents this limitation; everything
> else (health check, groups, time/current, the `ERR_AUTH_REQUIRED` path) still works exactly
> as described.

This iteration has no Telegram bot and no browser extension — the API is exercised directly
with `curl`. This is the current setup-to-boot walkthrough.

## 1. Prerequisites

- Go 1.23+
- Docker (for Postgres)

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

`INTERNAL_API_TOKEN` defaults to `dev` in `.env.example` — every `/api/v1/*` route except
`/healthz` requires it in the `X-Internal-Token` header. There is no encryption key to
generate any more — the server stores no credentials (see
[`docs/architecture/data-storage.md`](../architecture/data-storage.md) §3).

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

Covers week-parity math across year boundaries, subject normalization, and the merge
engine's matching/discard rules (`internal/engine`).

## 6. Smoke test

```bash
curl -s localhost:8080/healthz

curl -s -H "X-Internal-Token: dev" 'localhost:8080/api/v1/time/current'

curl -s -H "X-Internal-Token: dev" 'localhost:8080/api/v1/groups?query=ІП-54'

# unlinked/no-data user — expect ERR_AUTH_REQUIRED
curl -s -H "X-Internal-Token: dev" 'localhost:8080/api/v1/schedule/today?telegram_id=999'

curl -s -H "X-Internal-Token: dev" 'localhost:8080/api/v1/auth/status/999'   # NOT_LINKED

# missing/invalid token — expect 401
curl -s -o /dev/null -w '%{http_code}\n' 'localhost:8080/api/v1/time/current'
```

## 7. Getting a real schedule in — currently not possible via curl

There is deliberately no `POST /api/v1/auth/session` or `POST /api/v1/debug/mykpi/dump` any
more, and the browser extension's schedule-sync ingestion endpoint doesn't exist yet. Until
that endpoint is built, `user_lessons` can only be populated by inserting rows directly (e.g.
via `psql`) for manual `/schedule/*` testing — there is no supported end-to-end path from a
real `my.kpi.ua` account today.

## 8. Stop everything

```bash
docker compose down          # keeps the named volume (data survives)
docker compose down -v       # also deletes the volume
```
