# Project Repository & Technology Stack

## 1. Overview

This repository is a **monorepo** housing the complete ecosystem for the KPI Personalized Schedule service:

- **Golang Backend (`apps/server/`)**: The entire backend — REST API, Campus API client, merging engine, caching, persistence, the scheduler, **and the Telegram bot itself**. Never authenticates to `my.kpi.ua` or stores credentials — see [`docs/architecture/data-storage.md`](architecture/data-storage.md).
- **Browser Extension (`apps/extension/`)**: Manifest V3 extension (TypeScript) that fetches and parses the student's `my.kpi.ua` schedule client-side, using the browser's own session, and pushes the resulting lesson list to the backend. **Not yet built** — see [`docs/extension/browser-extension-design.md`](extension/browser-extension-design.md).
- **Documentation (`docs/`)**: Specifications, endpoint references, research findings, and architectural guidelines.

> **Status: provisional.** The directory layout below — in particular the internal package breakdown of `apps/server/` — is the **expected** structure, not a frozen contract. Packages may be merged, split, renamed, or dropped as implementation reveals what is actually needed. Whoever changes it **must update this file in the same task** (see [`CLAUDE.md`](../CLAUDE.md)).

---

## 2. Monorepo Directory Structure (expected)

```text
kpi-schedule-bot/
├── docs/                               # Project documentation (source of truth)
│   ├── project-repository.md           # This file: stack, structure, tooling
│   ├── architecture/                   # High-level architecture & algorithms
│   ├── schedules/                      # External source research (main / secondary)
│   ├── api/                            # Backend API specifications
│   ├── extension/                      # Browser extension architecture
│   └── bot/                            # Telegram bot specifications
│
└── apps/
    ├── server/                         # Go 1.23+ — the whole backend
    │   ├── compose.yaml                # Runs the built container against a local volume,
    │   │                                #   for testing the Dockerfile/persistence shape —
    │   │                                #   not required for day-to-day local dev (SQLite
    │   │                                #   needs no separate service)
    │   ├── .env.example
    │   ├── cmd/server/main.go          # Single entrypoint
    │   ├── internal/
    │   │   ├── config/                 # Env-var config, fails fast on missing secrets
    │   │   ├── api/                    # chi router, handlers, orchestration (Service)
    │   │   ├── campus/                 # Client for api.campus.kpi.ua (disk-cached via storage.DB)
    │   │   ├── engine/                 # Schedule merging, normalization, week-parity math
    │   │   ├── storage/                # SQLite repository (database/sql + modernc.org/sqlite)
    │   │   │                            #   + goose migrations + the campus_cache table
    │   │   ├── model/                  # Shared domain structs, error codes
    │   │   └── bot/                    # Telegram bot (gotgbot/v2), in-process — see
    │   │                                #   docs/bot/telegram-bot-design.md. /start, /link,
    │   │                                #   /today, /week implemented as inline-button
    │   │                                #   screens; rest still not built.
    │   ├── go.mod
    │   └── Dockerfile                  # Host-agnostic deployment artifact, VOLUME ["/data"]
    │                                    #   for the SQLite file
    │
    │   # Not yet created — deferred:
    │   #   internal/scheduler/  (morning reminders / stale-check worker, in-process cron via
    │   #                         github.com/go-co-op/gocron/v2 — see
    │   #                         docs/bot/telegram-bot-design.md §6; unrelated to the old
    │   #                         refresh-cron concept below, which is dropped entirely)
    │   #
    │   # Removed (architecture decision, see docs/architecture/data-storage.md):
    │   #   internal/mykpi/   (my.kpi.ua HTTP client/parser — now the extension's job)
    │   #   internal/crypto/  (AES-256-GCM cookie encryption — nothing left to encrypt)
    │
    └── extension/                      # TypeScript + Manifest V3
        ├── src/popup/                  # Popup UI
        ├── src/background/             # Service worker (cookie extraction & sync)
        ├── manifest.config.ts          # Manifest generated via @crxjs
        └── package.json
```

### 2.1 Deliberate omissions

These were considered and **intentionally left out** until a concrete need appears:

| Omitted | Rationale |
| :--- | :--- |
| Root `package.json` / Bun workspaces | Only one TypeScript package exists (`apps/extension`). Bun is used as the package manager *inside* that directory. |
| `packages/` (shared TS code) | The extension and the backend share no TypeScript. A shared package would be an empty abstraction. |
| `Makefile` | Its only role was unifying two toolchains; with a single Go binary and a single TS app, `go run` and `bun run` suffice. |
| `.github/` (CI/CD) | To be added deliberately when CI/CD is introduced, as a separate piece of work. |

---

## 3. Technology Stack

### 3.1 Backend (Golang)

- **Language**: Go 1.23+
- **HTTP Router**: `chi` (`github.com/go-chi/chi/v5`) — minimal overhead, readable middleware chaining.
- **Rate Limiting**: `github.com/go-chi/httprate`, 20 req/min per client IP on all `/api/v1/*` routes (except the Telegram webhook route `POST /api/v1/telegram/webhook`, which is exempt and authenticated via secret token) — see [`docs/architecture/error-handling-resilience.md`](architecture/error-handling-resilience.md) §5.
- **Telegram Bot**: `gotgbot/v2` (`github.com/PaulSonOfLars/gotgbot/v2`) — code-generated from the Bot API specification, fully type-safe, standard library only. Chosen because message *mutation* methods (`editMessageText`, `editMessageReplyMarkup`, `deleteMessage`, `answerCallbackQuery`) map 1:1 to the Bot API, which the inline-keyboard navigation depends on.
- **my.kpi.ua Client**: none — the server never fetches `my.kpi.ua`. That fetch (a small regexp extraction of the FullCalendar events URL embedded in the calendar shell page's inline script, then the JSON events feed itself; see `docs/schedules/main/data-extraction.md`) is now the browser extension's job, done client-side.
- **Persistence**: SQLite (`modernc.org/sqlite`, pure Go, CGO-free) via `database/sql`, on a mounted persistent-disk volume. A prior decision here rejected SQLite for ephemeral-disk deployment targets; the target host is now confirmed to have a persistent disk, and the host VM is meant to sleep when idle to save cost, which is exactly what makes SQLite the better fit — see `docs/architecture/data-storage.md` §5.
- **Scheduling**: no schedule-refresh cron — the server cannot refresh a schedule on its own since it has no credentials to fetch with; the only way data changes is a push from the extension (see `docs/architecture/data-storage.md` §4). The not-yet-built Telegram bot's morning-reminder worker (`internal/scheduler/`, see `docs/bot/telegram-bot-design.md` §6) is a separate, unrelated concern and may still use in-process cron (`github.com/go-co-op/gocron/v2`) when it's built.
- **Caching**: Disk-backed, in the same SQLite database (`campus_cache` table) — not in-memory. Group schedules TTL ~6h, catalog/slot/current-time data TTL 1min–24h, unchanged from the old in-memory cache's TTLs. Moved to disk specifically so a value cached before the VM sleeps is still warm on wake; see `docs/architecture/data-storage.md` §5. The in-process rate limiter (below) is unaffected and stays in-memory.
- **Logging**: Structured logging via `log/slog`.

### 3.2 Browser Extension

- **Platform**: WebExtensions Manifest V3 (Chrome, Brave, Edge, Firefox).
- **Language**: TypeScript.
- **Build**: Vite + `@crxjs/vite-plugin`; **Bun** as package manager.
- **Permissions**: `cookies`, `storage`, and `host_permissions` limited to `https://my.kpi.ua/*` and the backend origin.
- **Not yet built** — see [`docs/extension/browser-extension-design.md`](extension/browser-extension-design.md) for the target design: it fetches and parses the schedule itself, then pushes parsed lessons (not cookies) to the backend.

---

## 4. Architectural Decisions

### 4.1 Single Go service (no separate bot process)

A separate TypeScript bot (Cloudflare Workers / Node) was evaluated and **rejected**.

A Telegram bot is nothing more than an HTTPS client of `api.telegram.org` — there is no privileged runtime and no capability available to TypeScript that Go lacks. Because the browser extension already requires the Go server to be a publicly reachable HTTPS endpoint, a separate bot process would only add a network hop, a second language, a second deployment target, and a second location for secrets, in order to proxy requests to the Go server.

**Tradeoff accepted**: message formatting is somewhat more verbose in Go than in TypeScript. This is outweighed by eliminating an entire deployment target.

### 4.2 Production hosting target: Fly.io Fly Machines (Scale-to-Zero)

The production deployment target is **Fly.io Fly Machines** with scale-to-zero enabled:

1. Ships as a plain `Dockerfile` and deploys via `fly.toml`.
2. Reads all configuration from environment variables (`.env` locally, `fly secrets` / `fly.toml [env]` in production).
3. Persists SQLite to a mounted persistent volume (`fly volumes create kpi_data --size 1 --region waw`, mounted at `/data` via `VOLUME ["/data"]` and `fly.toml [mounts]`).
4. Sleeps after 15 minutes of inactivity (`IDLE_TIMEOUT=15m`) via in-app `internal/idle.Watcher` and exits with code 0 (`[[restart]] policy = "on-failure"` keeps the machine stopped).
5. Wakes up automatically when any request arrives at Fly Proxy (`auto_start_machines = true`). See [`docs/architecture/fly-scale-to-zero.md`](architecture/fly-scale-to-zero.md) for full architectural details.

### 4.3 Telegram update delivery
 
- **Delivery**: webhook → `POST /api/v1/telegram/webhook`, protected by `setWebhook`'s `secret_token` and verified against the `X-Telegram-Bot-Api-Secret-Token` header.
- **Testing**: in local development, an HTTPS tunnel (e.g. ngrok) exposes `:8080` to Telegram. Long polling is not used.

Note that *sending* messages (e.g. scheduled reminders) is an ordinary outbound HTTPS call to `api.telegram.org`.

---

## 5. Development Principles

1. **Simplicity Over Complexity**: Keep domain structures clean and flat. Avoid unnecessary abstractions — the omissions in §2.1 are the standard.
2. **Explicit Error Handling**: Wrap errors with context (`fmt.Errorf("fetching group lessons: %w", err)`) and degrade gracefully for the user.
3. **Resilience**: `api.campus.kpi.ua` is reliable and structured, but the server must still degrade gracefully (unenriched lessons, `enrichment_status: "degraded"`) if it's unreachable — see `docs/architecture/error-handling-resilience.md`. The server has no `my.kpi.ua` fetch of its own to worry about any more.
