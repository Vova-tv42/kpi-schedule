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
- The Machines API queries Fly's control plane directly and **never sends traffic through Fly Proxy or to the VM**, allowing real-time status checks (`awake`, `sleeping`, `transitioning`, `down`, `unconfigured`) with **zero wake-ups**.
- **Lifecycle & Initial Loading State**: The client status store (`server-status-store.svelte.ts`) initializes with status `'loading'`, displaying a neutral loading indicator (`LOADING...`) in the header rather than flashing false `VM OFFLINE` alerts while awaiting external signals. When Fly responds, the badge updates dynamically to `VM AWAKE`, `STANDBY (0/15m)`, `STARTING...`, or `UNCONFIGURED`. If Fly Machines API fails to respond within the 5-second timeout window or reports a down/failed state, it transitions cleanly to `VM OFFLINE`.

### 3.2 Server Sleep Interlock UX
- If the server is in standby (`sleeping`), the Database Tables and Custom Query pages do **not** auto-fetch on mount.
- Instead, a tactile **Scale-to-Zero Guard Banner** warns the admin:
  > *"The main server is currently in Scale-to-Zero standby on Fly.io to save compute billing. Proceeding will trigger a cold-boot (~500ms) and keep the machine awake for 15 minutes of idle time."*
- The admin must explicitly click `[Wake Server & Load Tables]` to proceed, or click `[Stay in Standby]` to return to the telemetry actions page (which reads purely from NeonDB).

---

## 4. Anonymous Action Telemetry & Retention Lifecycle

### 4.1 Non-Blocking Event Reporting
- Whenever the Go main server processes a Telegram command, callback, browser extension schedule sync, scheduled lesson alert, or mutating admin operation (table row updates, custom queries executed via the dashboard), it is already awake.
- An asynchronous background goroutine dispatches an anonymous event payload to `POST /api/ingest/action` on the admin dashboard with header `X-Ingest-Key`.
- **Supported Event Types**:
  - `telegram_command`: Bot command invocations (e.g. `/today`, `/week`, `/settings`).
  - `telegram_callback`: Inline button interactions.
  - `extension_sync`: Schedule pushes from the browser extension.
  - `cron_alert`: Periodic lesson reminder dispatches.
  - `admin_action`: Dashboard-initiated SQLite mutations (`update_row:<table_name>`) and console executions (`custom_query`).
- **Anonymity Guarantee**: All user-identifying attributes (Telegram IDs, user IDs, chat IDs, names, phone numbers, tokens) are strictly scrubbed before persistence.
- **Resilience**: The telemetry call has a 2s timeout and executes in a detached goroutine (`go func() { ... }()`). If the ingestion endpoint is unreachable, the main server silently drops the event and continues normal operations without delay.

### 4.2 Retention Window & Free Tier Cleanup
- Telemetry events are stored in NeonDB's `recent_actions` table.
- The retention window is configurable in the Settings tab (default: 72 hours).
- **Cleanup Triggers**:
  1. **Opportunistic Pruning**: During normal telemetry ingestion, a small percentage of requests automatically execute a quick `DELETE` of expired rows.
  2. **External Free Cron**: `POST /api/cron/cleanup` can be scheduled via `cron-jobs.com` or `cron-job.org` using `Authorization: Bearer <CRON_SECRET>` on an hourly, 6-hour, or daily basis.
  3. **Manual Trigger**: Administrators can trigger cleanup on demand from the Settings tab.

---

## 5. UI Architecture & Design System

The Admin Dashboard is built with a dual-theme design system: **Cyber-Industrial Mission Control** (Dark) and **Tactical Blueprint / High-Precision Telemetry Lab** (Light). Both themes maintain high information density, strict typographic hierarchy, and tactile telemetry aesthetics without generic washed-out grays.

### 5.1 Theme Engine & Mode Switching
- **Mode Management (`mode-watcher`)**: Built with `mode-watcher` (official Svelte 5 theme manager from `@svecosystem`). Injected at the root in `src/routes/+layout.svelte` via `<ModeWatcher defaultMode="dark" track={true} />`.
- **Zero-FOUC Guarantee**: Applies theme class directly to the document head before render, preventing flash of unstyled content. Persists admin preference in `localStorage` under `mode-watcher-mode`.
- **Tailwind CSS v4 Compatibility**: Configured in `src/routes/layout.css` using the `@custom-variant dark (&:where(.dark, .dark *));` directive, synchronizing Tailwind v4 with `mode-watcher`'s HTML class toggling.
- **Interactive Switcher (`ThemeToggle.svelte`)**: A segmented, tactile toggle button mounted in both the authenticated `TelemetryHeader` and the unauthenticated `/login` page. Uses CSS-driven visibility toggling (`:global(.dark)`) rather than client-side JS conditional rendering (`{#if mode.current === 'dark'}`), ensuring the active icon/label instantly matches the head-injected theme class on first paint without hydration flicker.

### 5.2 Layout & Navigation Hierarchy
- **Left Sidebar Rail (`Navbar.svelte`)**: Fixed 64-width navigation bar (`bg-white dark:bg-[#0e1117]`, border `border-[#d2d7e2] dark:border-[#252b3b]`) with operational pulse indicator and categorized links:
  - **Telemetry**: `Overview` (`/`), `Action Stream` (`/actions`).
  - **Storage Engine**: `Tables & Rows` (`/database`), `SQL Workspace` (`/database/query`).
  - **Governance**: `Admin Access` (`/admins`, locked for non-superadmins), `Settings` (`/settings`).
  - **Mobile Support**: Off-canvas sliding drawer with adaptive backdrop overlay for viewports `< 1024px`.
- **Top Diagnostics Header (`TelemetryHeader.svelte`)**: Sticky 14-height bar (`bg-white/80 dark:bg-[#0e1117]/80 backdrop-blur-md`, border `border-[#d2d7e2] dark:border-[#252b3b]`):
  - Safe Fly.io VM status indicator (`awake`, `sleeping`, `transitioning`, `loading`, `unconfigured`, `down`) with non-waking Machines API refresh button.
  - Theme mode toggle button (`ThemeToggle.svelte`).
  - Authenticated user email and role badge (`SUPERADMIN` in lime, `READ & WRITE` in cyan, `READ ONLY` in amber).
  - Secure disconnect session termination action.
- **Main Operations Viewport**: High-density workspace rendered inside an overflow scroll container with custom adaptive hairline scrollbars and technical background grids (`.bg-tech-grid`).

### 5.3 Design Tokens & Aesthetic Modes

| Token Category | Dark Mode ("Mission Control") | Light Mode ("Tactical Blueprint") |
| :--- | :--- | :--- |
| **Canvas Background** | Deep Cyber-Black (`#0a0b0e`) | Warm Drafting Paper (`#f4f5f8`) |
| **Panels & Cards** | Deep Space (`#12151d`) | Surgical White (`#ffffff`) |
| **Inset Surfaces** | Inset Well (`#181c26`) | Inset Tint (`#f0f2f6`) |
| **Table & Modal Headers** | Header Dark (`#151922`) | Header Blueprint (`#f8fafc` / `#eef2f7`) |
| **Structural Borders** | Slate Gridline (`#252b3b`) | Blueprint Border (`#d2d7e2`) |
| **Primary Typography** | Crisp Off-White (`#f1f5f9` / `#ffffff`) | Deep Drafting Ink (`#090d16`) |
| **Muted Typography** | Cool Gray (`#94a3b8` / `#64748b`) | Engineering Slate (`#475569` / `#64748b`) |
| **Primary Accent (Volt)** | Neon Volt (`#d4ff32`) with `#090d16` text | Tactical Volt (`#ccf600`) with `#090d16` text, solid border & hard drafting shadow |
| **Background Grid** | 24px Radar Grid (`rgba(255,255,255,0.02)`) | 24px Millimeter Drafting Grid (`rgba(0,0,0,0.035)`) |

#### Typography Stack
- Display Font: `Bricolage Grotesque` (bold uppercase headers, branding, section badges).
- Body Font: `Geist` (clean modern sans for tabular data and general UI).
- Monospace Font: `JetBrains Mono` (SQL editor, JSON payloads, metrics, timestamps, and row tables).

### 5.4 Reusable Primitives
- `Badge.svelte`: Semantic status badges supporting `lime`, `amber`, `emerald`, `crimson`, `slate`, and `cyan`. Colors are tuned with distinct light-mode washes and high-contrast ink text to guarantee WCAG AA readability.
- `ThemeToggle.svelte`: Tactile toggle switch for changing between Dark and Light color schemes with smooth SVG icon transitions.
- `Modal.svelte`: Accessible dialog overlay with Esc key handling, backdrop blur, adaptive card backgrounds, and technical header with volt accent dot.
- `ServerInterlockModal.svelte`: Specialized Scale-to-Zero warning dialog preventing unintentional cold boots of the Fly.io microVM, styled with industrial hazard stripes and tactile confirmation buttons.

