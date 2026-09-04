# KPI Schedule

A Telegram bot that gives KPI students their *personal* class schedule — only the electives
and subgroups they are actually enrolled in, enriched with rooms, lecturers and campus map
links, with reminders before each class.

The system is three apps in one repo:

| App | Stack | Role |
| :-- | :-- | :-- |
| [`apps/server`](apps/server) | Go 1.26, chi, SQLite, gotgbot | REST API, merging engine, and the Telegram bot itself — one binary |
| [`apps/extension`](apps/extension) | TypeScript, Vite, Manifest V3 | Reads the student's schedule from `my.kpi.ua` in their own browser and pushes it to the server |
| [`apps/admin`](apps/admin) | SvelteKit, Tailwind v4, Postgres (Neon) | Operator dashboard: anonymous telemetry, DB inspection, admin access control |

---

## Why

KPI has two schedule sources, and neither is enough on its own:

- **`my.kpi.ua`** knows *which* classes a student actually attends (their electives, their
  subgroup) — but it's behind a session login and carries no rooms, lecturers or map links.
- **`api.campus.kpi.ua`** is public and rich in metadata — but it returns the whole group's
  timetable, including every elective and subgroup the student never attends.

The server merges them: the personal feed decides *what* and *when*, the public API fills in
*where* and *with whom*. The result is a clean timetable in Telegram, with no login step and
no noise.

## How it works

```
  Student's browser                    Server (Fly.io)              Telegram
  ┌────────────────────┐               ┌───────────────────┐
  │ my.kpi.ua session  │               │ POST /schedule/   │
  │        ↓           │  parsed       │      sync         │
  │ Extension fetches  │  lessons      │        ↓          │        /today
  │ + parses           ├──────────────►│ merge + enrich    │◄────── /week    ── Student
  └────────────────────┘  (JSON)       │        ↓          │        /group
                                       │ SQLite  ──────────┼──────► reminders
                                       └────────▲──────────┘
                                                │ rooms, lecturers, maps
                                       api.campus.kpi.ua
```

1. The student runs `/link` in the bot and gets a 6-digit, single-use code (10 min TTL).
2. They enter it in the extension once — this proves they own that Telegram account.
3. With `my.kpi.ua` open in their browser, they hit "Sync". The extension fetches and parses
   the schedule **client-side**, then POSTs the resulting lesson list to the server.
4. The server matches each lesson against the public Campus timetable, adds room / lecturer /
   map data, and stores it. Everything after that (`/today`, `/week`, alerts) reads from the
   database — no external fetch, no re-login.

## Privacy: the extension only ever sends the schedule

This is the whole reason the extension exists. An earlier design had the server accept
`my.kpi.ua` session cookies and scrape on the student's behalf; that was dropped.

- **No credentials leave the browser.** No password, no `PHPSESSID`, no `_identity` cookie,
  no bearer token from `my.kpi.ua` is ever transmitted or stored server-side. The extension
  doesn't even hold the `cookies` permission — it requests `storage` and nothing else.
- **The only payload is a lesson list**: date, start/end time, subject, type (lec/prac/lab),
  teacher name, room string. That's it.
- **The server cannot act as the student.** It has no way to reach `my.kpi.ua`; the schedule
  changes only when the student pushes it. There is no credential store to leak, and no
  encryption-at-rest problem, because there's nothing to encrypt.
- **No trackers.** No analytics SDKs in the extension; server-side telemetry is event counts
  with all identifiers scrubbed before they're stored.

See [`docs/architecture/data-storage.md`](docs/architecture/data-storage.md) for the full
persistence policy.

---

## Apps

### Server (`apps/server`)

A single Go binary containing the REST API, the Campus API client, the merging engine, SQLite
persistence, and the Telegram bot (webhook-based, no separate process). Deployed to Fly.io
with scale-to-zero: it sleeps after 15 minutes idle, wakes on the next request, and keeps its
SQLite file on a mounted volume. Lesson reminders are driven by an external cron service
hitting `/api/v1/cron/lesson-alerts`, since in-process timers can't fire while the VM sleeps.

Bot commands: `/today`, `/week`, `/urls` (conference links per subject), `/group` (bind an
academic group to a Telegram chat), `/group_today`, `/group_week`, `/settings`, `/link`,
`/install`.

### Extension (`apps/extension`)

Manifest V3, TypeScript, no framework. A popup, a service worker, and the fetch/parse logic.
Host permissions are limited to `my.kpi.ua`, `api.campus.kpi.ua`, and the backend origin. The
backend URL is a build-time env var with a runtime override in the popup, so self-hosted
instances work. Distributed as a zip (`bun run build:zip`) or via the Chrome Web Store.

### Admin dashboard (`apps/admin`)

SvelteKit on Vercel, deliberately separate from the Go server so it stays reachable while the
server sleeps — it reads Fly machine state through the Machines API, which doesn't wake it.
Google OAuth (PKCE) with a strict email whitelist; rejected logins create no session and no
row. Provides an action stream of anonymous telemetry, a browser and SQL console for the
server's SQLite tables (proxied, secret-authenticated), and role-based admin management.

---

## Running locally

**Server** — SQLite needs no external service:

```bash
cd apps/server
cp .env.example .env      # defaults work as-is for API-only mode
go run ./cmd/server       # :8080
go test ./...
```

Leave `TELEGRAM_BOT_TOKEN` empty to run the API without the bot. To run the bot, set the
token and point `TELEGRAM_WEBHOOK_URL` at an HTTPS tunnel (e.g. ngrok) to `:8080`.

**Extension**:

```bash
cd apps/extension
bun install
bun run build             # → dist/, load unpacked at chrome://extensions
```

Set `VITE_BACKEND_URL=http://localhost:8080` in `.env` to target a local server.

**Admin dashboard**:

```bash
cd apps/admin
bun install
bun run db:up             # local Postgres in Docker, port 5435
bun run dev               # :5173
```

Requires Google OAuth credentials and `SUPERADMIN_EMAIL` in `.env` — see `.env.example`.

## Deployment

Pushes to `main` touching `apps/server/**` run the Go tests and deploy to Fly.io via GitHub
Actions ([`.github/workflows/fly-deploy.yml`](.github/workflows/fly-deploy.yml)). The admin
dashboard deploys from Vercel; the extension is packaged and published manually.

## Documentation

[`docs/`](docs/README.md) is the source of truth for architecture decisions, the external KPI
systems' behaviour, API contracts, and the merging algorithm. Start with
[`docs/architecture/overview.md`](docs/architecture/overview.md).
