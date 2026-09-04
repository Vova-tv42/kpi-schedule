# KPI Personalized Schedule Documentation

Welcome to the technical documentation for the **KPI Personalized Schedule Platform**.

---

## Documentation Directory Index

### 📁 General & Repository Setup
- [Project Repository & Tech Stack](file:///home/volodymyr/apps/kpi-schedule-bot/docs/project-repository.md): Monorepo structure, Golang libraries, and developer guidelines.

### 📁 System Architecture
- [Architecture Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/overview.md): High-level system design and end-to-end data flow.
- [Schedule Merging Engine](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/merging-engine.md): Matching algorithm, date filtering (`dates: [...]`), and course deduplication.
- [Error Handling & Resilience](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/error-handling-resilience.md): Stale-push handling, Campus API circuit breaking, and the standard error envelope.
- [Data Storage & Sync Policy](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/data-storage.md): SQLite schema (including the disk-backed `campus_cache` table), why no credentials are ever stored, and the extension-push sync model.
- [Fly.io Scale-to-Zero & 15m Idle](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/fly-scale-to-zero.md): Firecracker microVM scale-to-zero, in-app 15m idle shutdown, wake-on-request proxy, and volume persistence.
- [CI/CD Deployment via GitHub Actions](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/ci-cd-deployment.md): Automated tests, flyctl deployment on main branch pushes, and deploy token configuration.
- [Notifications & Scheduled Cron](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/notifications-and-cron.md): Automated lesson alerts (10m before and at start), idempotency, and cron-job.org setup.


### 📁 Schedule Sources
- **Main Schedule (`my.kpi.ua`)**:
  - [Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/overview.md): System architecture, Yii2 PHP stack, selective courses & subgroups.
  - [Authentication](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/auth.md): `PHPSESSID`, `_identity`, `_csrf`, and login mechanisms.
  - [Data Extraction](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/data-extraction.md): the FullCalendar JSON events feed, its two-step discovery flow, and field mapping.
  - [Browser Extension Auth Flow](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/extension-auth-flow.md): Step-by-step pairing handshake protocol.
- **Secondary / Group Schedule (`schedule.kpi.ua` / `api.campus.kpi.ua`)**:
  - [Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/secondary/overview.md): Public React SPA and backend architecture.
  - [API Endpoints Reference](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/secondary/endpoints.md): Full reference for all public `/schedule/*` and `/time/*` endpoints.
  - [Data Models & Schemas](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/secondary/data-models.md): JSON structures and Go struct definitions.

### 📁 Golang Backend API
- [API Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/overview.md): REST routing, conventions, and status codes.
- [Auth Endpoints](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/auth-endpoints.md): Link status checks and unlinking — no credentials to link/store any more.
- [Admin Endpoints](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/admin-endpoints.md): Database inspection, row editing, and custom query endpoints for the admin dashboard.
- [Schedule Endpoints](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/schedule-endpoints.md): Queries for `/today`, `/tomorrow`, `/week`, and specific dates.
- [Local Development & Manual Testing](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/local-development.md): No-Docker-needed SQLite setup, `.env` configuration, the `curl` walkthrough, and testing the Docker deployment shape (no bot/extension yet, and no way to push a real schedule until the sync endpoint exists).

### 📁 Client Applications
- [Browser Extension (Manifest V3)](file:///home/volodymyr/apps/kpi-schedule-bot/docs/extension/browser-extension-design.md): Extension design, permissions, and security.
- [Telegram Bot](file:///home/volodymyr/apps/kpi-schedule-bot/docs/bot/telegram-bot-design.md): Bot commands, UI formatters, inline keyboards, and reminders.
