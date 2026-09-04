# Admin Web Dashboard Architecture & Security Model

This document specifies the architecture, hosting, security mechanisms, and telemetry pipeline for the **KPI Schedule Admin Web Dashboard** located in `apps/admin`.

---

## 1. High-Level Architecture & Deployment Model

```
                                      ┌──────────────────────────────────────┐
                                      │         Google Identity Provider     │
                                      │            (OAuth 2.0 PKCE)          │
                                      └──────────────────┬───────────────────┘
                                                         │
                                                         ▼
┌────────────────────────┐              ┌────────────────────────────────────┐
│      Admin Browser     │ ◄──────────► │       SvelteKit Admin Portal       │
│  (Console UI with TW)  │  Auth Cookie │        (Deployed on Vercel)        │
└────────────────────────┘              └───────┬────────────┬─────────────┬─┘
                                                │            │             │
                       Query Machine State      │            │             │ Manage Admins,
                       (Bypasses Fly Proxy,     │            │             │ Whitelist, Sessions,
                        Never wakes microVM)    ▼            │             ▼ Retention & Actions
                                       ┌────────────────┐    │      ┌──────────────┐
                                       │  Fly Machines  │    │      │    NeonDB    │
                                       │      API       │    │      │ (Serverless  │
                                       │(api.machines.  │    │      │  PostgreSQL) │
                                       │     dev)       │    │      └──────▲───────┘
                                       └────────────────┘    │             │
                                                             │             │
                                   Admin Proxy Requests      │             │
                                   (Authenticated by Secret, │             │
                                    Triggers cold boot)      ▼             │
                                               ┌──────────────────────┐    │ Async Telemetry
                                               │    Go Main Server    ├────┘ (Non-blocking POST,
                                               │ (kpi-schedule.fly.dev│      Anonymous metadata)
                                               │  Scale-to-Zero 15m)  │
                                               └──────────────────────┘
```

The Admin Web Dashboard is hosted independently on **Vercel** (`apps/admin`), decoupled from the main Go server (`apps/server`) on Fly.io. This guarantees:
1. The admin interface is always online and accessible, even while the main server is completely powered down.
2. The main server does not spend CPU cycles rendering web pages or hosting dashboard assets.
3. Secondary features (action logs, session management, admin accounts) live in an external database (**NeonDB** in production or **Docker PostgreSQL** in local development), preventing bloat on the main server's disk volume.

### 1.1 Dual Database Adapter Support
The admin database client (`src/lib/server/db.ts`) dynamically detects the database target based on the `DATABASE_URL`:
- **Local Development**: When `DATABASE_URL` references `localhost`, `127.0.0.1`, or a local port (e.g. port 5435), it uses the standard TCP `postgres` driver connected to the local Docker container (`kpi-admin-postgres`). Run `bun run db:up` to start the local container.
- **Production on Vercel**: When `DATABASE_URL` references `neon.tech`, it uses `@neondatabase/serverless` over HTTP/WebSocket connection pooling, optimized for serverless edge/lambda execution without persistent TCP connection limits.

---

## 2. Authentication & Authorization Security Model

### 2.1 Google OAuth 2.0 PKCE Only
- No passwords, magic links, or username-based logins exist. Only Google OAuth 2.0 with Proof Key for Code Exchange (PKCE) is allowed.
- Powered by `arctic` for runtime-agnostic security.

### 2.2 Strict Whitelist Enforcement (Zero Storage on Rejection)
When a Google account authenticates:
1. The email is normalized and checked against `SUPERADMIN_EMAIL` (configured in secret variables).
2. If not the superadmin, the email is checked against the `admin_users` table in NeonDB.
3. **Strict Rejection Policy**: If the email is neither the superadmin nor an approved secondary admin:
   - Access is immediately blocked (`403 Forbidden`).
   - **Zero data is stored**: No session is created, no cookie is set, and no record is added to the database.
   - The user is redirected to `/login?error=forbidden`.

### 2.3 Role-Based Access Control (RBAC)
The system defines three privilege tiers:

| Capability | `superadmin` | `read-write` | `read-only` |
| :--- | :---: | :---: | :---: |
| View Recent Actions Telemetry | ✅ | ✅ | ✅ |
| View SQLite Database Tables & Rows | ✅ | ✅ | ✅ |
| Edit Database Table Rows | ✅ | ✅ | ❌ |
| Run Custom SQL Queries | ✅ | ✅ | ❌ |
| Add / Update / Delete Admins | ✅ | ❌ | ❌ |
| Adjust Retention Settings | ✅ | ✅ | ❌ |
| Trigger Manual Cleanup | ✅ | ✅ | ✅ |

- **Protection of Custom Queries**: For `read-only` users, the Custom Query console is completely disabled in the UI and blocked at the API layer with `403 Forbidden`.
- **Immediate Session Revocation**: When an admin's role is updated or their account is deleted by the superadmin, all their active sessions in `admin_sessions` are immediately purged.

---

## 3. Fly.io Scale-to-Zero Integration & Wake Prevention

### 3.1 Non-Waking Status Checks via Fly Machines API
- Standard HTTP calls to `kpi-schedule.fly.dev` route through Fly Proxy, which immediately triggers a microVM cold boot.
- To monitor server health without waking it, the dashboard queries Fly.io's control plane:
  `GET https://api.machines.dev/v1/apps/{app_name}/machines` with `Authorization: Bearer <FLY_API_TOKEN>`.
- The Machines API queries Fly's control plane directly and **never sends traffic through Fly Proxy or to the VM**, allowing real-time status checks (`awake`, `sleeping`, `down`) with **zero wake-ups**.

### 3.2 Server Sleep Interlock UX
- If the server is in standby (`sleeping`), the Database Tables and Custom Query pages do **not** auto-fetch on mount.
- Instead, a tactile **Scale-to-Zero Guard Banner** warns the admin:
  > *"The main server is currently in Scale-to-Zero standby on Fly.io to save compute billing. Proceeding will trigger a cold-boot (~500ms) and keep the machine awake for 15 minutes of idle time."*
- The admin must explicitly click `[Wake Server & Load Tables]` to proceed, or click `[Stay in Standby]` to return to the telemetry actions page (which reads purely from NeonDB).

---

## 4. Anonymous Action Telemetry & Retention Lifecycle

### 4.1 Non-Blocking Event Reporting
- Whenever the Go main server processes a Telegram command, callback, browser extension schedule sync, or scheduled lesson alert, it is already awake.
- An asynchronous background goroutine dispatches an anonymous event payload to `POST /api/ingest/action` on the admin dashboard with header `X-Ingest-Key`.
- **Anonymity Guarantee**: All user-identifying attributes (Telegram IDs, user IDs, chat IDs, names, phone numbers, tokens) are strictly scrubbed before persistence.
- **Resilience**: The telemetry call has a 1.5s timeout and executes in a detached goroutine (`go func() { ... }()`). If the ingestion endpoint is unreachable, the main server silently drops the event and continues normal operations without delay.

### 4.2 Retention Window & Free Tier Cleanup
- Telemetry events are stored in NeonDB's `recent_actions` table.
- The retention window is configurable in the Settings tab (default: 72 hours).
- **Cleanup Triggers**:
  1. **Opportunistic Pruning**: During normal telemetry ingestion, a small percentage of requests automatically execute a quick `DELETE` of expired rows.
  2. **External Free Cron**: `POST /api/cron/cleanup` can be scheduled via `cron-jobs.com` or `cron-job.org` using `Authorization: Bearer <CRON_SECRET>` on an hourly, 6-hour, or daily basis.
  3. **Manual Trigger**: Administrators can trigger cleanup on demand from the Settings tab.
