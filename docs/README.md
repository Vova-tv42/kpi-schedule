# KPI Personalized Schedule Documentation

Welcome to the technical documentation for the **KPI Personalized Schedule Platform**.

---

## Documentation Directory Index

### 📁 General & Repository Setup
- [Project Repository & Tech Stack](file:///home/volodymyr/apps/kpi-schedule-bot/docs/project-repository.md): Monorepo structure, Golang libraries, and developer guidelines.

### 📁 System Architecture
- [Architecture Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/overview.md): High-level system design and end-to-end data flow.
- [Schedule Merging Engine](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/merging-engine.md): Matching algorithm, date filtering (`dates: [...]`), and course deduplication.
- [Error Handling & Resilience](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/error-handling-resilience.md): Session expiry, DOM change recovery, circuit breakers, and alert systems.
- [Data Storage, Encryption & Refresh Policy](file:///home/volodymyr/apps/kpi-schedule-bot/docs/architecture/data-storage.md): Schema, AES-256-GCM cookie encryption, the no-cron refresh policy, and the `dates[]` staleness guard.

### 📁 Schedule Sources
- **Main Schedule (`my.kpi.ua`)**:
  - [Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/overview.md): System architecture, Yii2 PHP stack, selective courses & subgroups.
  - [Authentication](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/auth.md): `PHPSESSID`, `_identity`, `_csrf`, and login mechanisms.
  - [Data Extraction](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/data-extraction.md): HTML scraping strategy, DOM selectors, and `goquery` parser.
  - [Browser Extension Auth Flow](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/main/extension-auth-flow.md): Step-by-step pairing handshake protocol.
- **Secondary / Group Schedule (`schedule.kpi.ua` / `api.campus.kpi.ua`)**:
  - [Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/secondary/overview.md): Public React SPA and backend architecture.
  - [API Endpoints Reference](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/secondary/endpoints.md): Full reference for all public `/schedule/*` and `/time/*` endpoints.
  - [Data Models & Schemas](file:///home/volodymyr/apps/kpi-schedule-bot/docs/schedules/secondary/data-models.md): JSON structures and Go struct definitions.

### 📁 Golang Backend API
- [API Overview](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/overview.md): REST routing, conventions, and status codes.
- [Auth Endpoints](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/auth-endpoints.md): Pairing codes, session synchronization, and status checks.
- [Schedule Endpoints](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/schedule-endpoints.md): Queries for `/today`, `/tomorrow`, `/week`, and specific dates.
- [Local Development & Manual Testing](file:///home/volodymyr/apps/kpi-schedule-bot/docs/api/local-development.md): Docker Compose setup, `.env` configuration, and the full `curl` walkthrough for manual testing (no bot/extension yet).

### 📁 Client Applications
- [Browser Extension (Manifest V3)](file:///home/volodymyr/apps/kpi-schedule-bot/docs/extension/browser-extension-design.md): Extension design, permissions, and security.
- [Telegram Bot](file:///home/volodymyr/apps/kpi-schedule-bot/docs/bot/telegram-bot-design.md): Bot commands, UI formatters, inline keyboards, and reminders.
