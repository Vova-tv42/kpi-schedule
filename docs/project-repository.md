# Project Repository & Technology Stack

## 1. Overview

This repository is a **monorepo** housing the complete ecosystem for the KPI Personalized Schedule service:

- **Golang Backend (`apps/server/`)**: The entire backend — REST API, `my.kpi.ua` scraper, Campus API client, merging engine, caching, persistence, the scheduler, **and the Telegram bot itself**.
- **Browser Extension (`apps/extension/`)**: Manifest V3 extension (TypeScript) that exports `my.kpi.ua` session cookies to the backend via a one-time pairing code.
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
    │   ├── compose.yaml                # Local Postgres for development
    │   ├── .env.example
    │   ├── cmd/server/main.go          # Single entrypoint
    │   ├── internal/
    │   │   ├── config/                 # Env-var config, fails fast on missing secrets
    │   │   ├── api/                    # chi router, handlers, orchestration (Service)
    │   │   ├── campus/                 # Client for api.campus.kpi.ua (TTL-cached)
    │   │   ├── mykpi/                  # Scraper for my.kpi.ua (goquery)
    │   │   ├── engine/                 # Schedule merging, normalization, week-parity math
    │   │   ├── storage/                # PostgreSQL repository (pgx) + goose migrations
    │   │   ├── cache/                  # Generic in-memory TTL cache
    │   │   ├── crypto/                 # AES-256-GCM cookie encryption
    │   │   └── model/                  # Shared domain structs, error codes
    │   ├── go.mod
    │   └── Dockerfile                  # Host-agnostic deployment artifact
    │
    │   # Not yet created — deferred, see docs/api/overview.md's "Note on the bot":
    │   #   internal/bot/        (Telegram bot; API/cookies/engine are bot-ready)
    │   #   internal/scheduler/  (cron; refresh is currently lazy-on-read, see
    │   #                         docs/architecture/data-storage.md §4)
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
- **Telegram Bot**: `gotgbot/v2` (`github.com/PaulSonOfLars/gotgbot/v2`) — code-generated from the Bot API specification, fully type-safe, standard library only. Chosen because message *mutation* methods (`editMessageText`, `editMessageReplyMarkup`, `deleteMessage`, `answerCallbackQuery`) map 1:1 to the Bot API, which the inline-keyboard navigation depends on.
- **HTML Parsing**: `goquery` (`github.com/PuerkitoBio/goquery`) for CSS-selector querying of `my.kpi.ua`.
- **Persistence**: PostgreSQL via `pgx`. **Not SQLite** — deployment targets have ephemeral disks.
- **Scheduling**: In-process cron (`github.com/go-co-op/gocron/v2`). No platform-level cron dependency, so the hosting choice stays open.
- **Caching**: In-memory cache; group schedules TTL ~24h, personal schedules TTL 1–6h.
- **Logging**: Structured logging via `log/slog`.

### 3.2 Browser Extension

- **Platform**: WebExtensions Manifest V3 (Chrome, Brave, Edge, Firefox).
- **Language**: TypeScript.
- **Build**: Vite + `@crxjs/vite-plugin`; **Bun** as package manager.
- **Permissions**: `cookies`, `storage`, and `host_permissions` limited to `https://my.kpi.ua/*` and the backend origin.

---

## 4. Architectural Decisions

### 4.1 Single Go service (no separate bot process)

A separate TypeScript bot (Cloudflare Workers / Node) was evaluated and **rejected**.

A Telegram bot is nothing more than an HTTPS client of `api.telegram.org` — there is no privileged runtime and no capability available to TypeScript that Go lacks. Because the browser extension already requires the Go server to be a publicly reachable HTTPS endpoint, a separate bot process would only add a network hop, a second language, a second deployment target, and a second location for secrets, in order to proxy requests to the Go server.

**Tradeoff accepted**: message formatting is somewhat more verbose in Go than in TypeScript. This is outweighed by eliminating an entire deployment target.

### 4.2 Host-agnostic backend

The hosting platform is **deliberately undecided**. To keep it open, the server must:

1. Ship as a plain `Dockerfile` with no platform-specific configuration.
2. Read all configuration from environment variables.
3. Use a network-attached PostgreSQL instance (no local disk state).
4. Run its own in-process scheduler rather than relying on platform cron.

### 4.3 Telegram update delivery

- **Production**: webhook → `POST /api/v1/telegram/webhook`, protected by `setWebhook`'s `secret_token` and verified against the `X-Telegram-Bot-Api-Secret-Token` header.
- **Local development**: long polling (`bot.Start()`), requiring no public URL or tunnel.

Both modes drive identical handler code. Note that *sending* messages (e.g. scheduled reminders) is an ordinary outbound HTTPS call and is unaffected by which delivery mode is active.

---

## 5. Development Principles

1. **Simplicity Over Complexity**: Keep domain structures clean and flat. Avoid unnecessary abstractions — the omissions in §2.1 are the standard.
2. **Explicit Error Handling**: Wrap errors with context (`fmt.Errorf("fetching group lessons: %w", err)`) and degrade gracefully for the user.
3. **Resilience**: `api.campus.kpi.ua` is reliable and structured. `my.kpi.ua` is server-rendered HTML; scrapers must fail safely with clear alerts and never crash the server.
