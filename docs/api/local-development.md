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

- Go 1.26+
- No Docker, no external services — SQLite is just a file (Docker is only needed to test the
  actual deployment container, see §9).

## 2. Configure environment

```bash
cd apps/server
cp .env.example .env
```

`DATABASE_PATH` defaults to `./data/kpi.db` in `.env.example` — the directory is created
automatically on first run if missing. `INTERNAL_API_TOKEN` defaults to `dev` — every
`/api/v1/*` route except `/healthz` requires it in the `X-Internal-Token` header. There is no
encryption key to generate any more — the server stores no credentials (see
[`docs/architecture/data-storage.md`](../architecture/data-storage.md) §3).

## 3. Run the server

```bash
go run ./cmd/server
```

`config.Load()` calls `godotenv.Load()` on startup, which reads `apps/server/.env` into the
process environment automatically (best-effort — it's a no-op if the file is absent, which is
the case in Docker/production where real env vars are set directly). No manual `source`/`export`
step needed.

Migrations apply automatically on startup (`internal/storage/migrations/`, via `goose`,
idempotent — a restart with the same `DATABASE_PATH` just logs "no migrations to run"). The
server listens on `HTTP_ADDR` (`:8080` by default). The Campus API cache
(`campus_cache` table, see `docs/architecture/data-storage.md` §5) persists in the same file,
so a restart against an existing `DATABASE_PATH` reuses whatever's still fresh instead of
re-fetching from `api.campus.kpi.ua`.

## 4. Run the test suite

```bash
go test ./...
```

Covers week-parity math across year boundaries, subject normalization, and the merge
engine's matching/discard rules (`internal/engine`).

## 5. Smoke test

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

## 6. Getting a real schedule in — currently not possible via curl

There is deliberately no `POST /api/v1/auth/session` or `POST /api/v1/debug/mykpi/dump` any
more, and the browser extension's schedule-sync ingestion endpoint doesn't exist yet. Until
that endpoint is built, `user_lessons` can only be populated by inserting rows directly (e.g.
via a Go script against the `DATABASE_PATH` file) for manual `/schedule/*` testing — there is
no supported end-to-end path from a real `my.kpi.ua` account today.

## 7. Testing the actual deployment shape (Docker + persistent volume)

Day-to-day development doesn't need this — it's for verifying the `Dockerfile` and the
persistent-volume setup the target host will use (see
[`docs/architecture/data-storage.md`](../architecture/data-storage.md) §5).

The container is published on host port **8081**, not 8080 — that's reserved for `go run
./cmd/server` (§3) so the two can run at the same time without a port conflict. Inside the
container it's still 8080 (matches the Dockerfile's `EXPOSE` and production).

```bash
docker compose up -d --build
docker compose ps
curl -s localhost:8081/healthz

docker compose restart server   # simulates the VM sleeping/waking
curl -s -H "X-Internal-Token: dev" 'localhost:8081/api/v1/groups?query=ІП-54'
# should respond fast — served from the persisted campus_cache, not a fresh Campus API fetch

docker compose down             # keeps the named volume (data survives)
docker compose down -v          # also deletes the volume
```
