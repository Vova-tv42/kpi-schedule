# Project Repository & Technology Stack

## 1. Overview

This repository is structured as a **monorepo** housing the complete ecosystem for the KPI Personalized Schedule service:
- **Golang Backend API Server**: Core business logic, schedule scrapers, campus API client, merging engine, caching, and database storage.
- **Telegram Bot**: User-facing bot providing commands, inline buttons, formatted schedule views, and automated notifications.
- **Browser Extension (Manifest V3)**: Client-side extension for Chromium/Firefox browsers to securely export session tokens from `my.kpi.ua` to the backend.
- **Documentation (`docs/`)**: Specifications, endpoint references, research findings, and architectural guidelines.

---

## 2. Monorepo Directory Structure

```text
kpi-schedule-bot/
├── docs/                               # Project documentation
│   ├── project-repository.md           # This file: stack, structure, tooling
│   ├── architecture/                   # High-level architecture & algorithms
│   │   ├── overview.md
│   │   ├── merging-engine.md
│   │   └── error-handling-resilience.md
│   ├── schedules/
│   │   ├── main/                       # Personal schedule (my.kpi.ua)
│   │   │   ├── overview.md
│   │   │   ├── auth.md
│   │   │   ├── data-extraction.md
│   │   │   └── extension-auth-flow.md
│   │   └── secondary/                  # Group schedule (api.campus.kpi.ua)
│   │       ├── overview.md
│   │       ├── endpoints.md
│   │       └── data-models.md
│   ├── api/                            # Backend API specifications
│   │   ├── overview.md
│   │   ├── auth-endpoints.md
│   │   └── schedule-endpoints.md
│   ├── extension/                      # Browser extension architecture
│   │   └── browser-extension-design.md
│   └── bot/                            # Telegram bot specifications
│       └── telegram-bot-design.md
│
├── cmd/                                # Entry points
│   ├── server/                         # Main API backend entrypoint (main.go)
│   └── bot/                            # Telegram Bot entrypoint (or embedded worker)
│
├── internal/                           # Private application code
│   ├── api/                            # HTTP routes, middleware, handlers
│   ├── bot/                            # Telegram bot handlers, keyboards, formatters
│   ├── campus/                         # Client for https://api.campus.kpi.ua
│   ├── mykpi/                          # Scraper and client for https://my.kpi.ua
│   ├── engine/                         # Schedule merging & enrichment logic
│   ├── storage/                        # Database repository (PostgreSQL / SQLite)
│   ├── cache/                          # In-memory / Redis cache layer
│   └── model/                          # Shared domain structs and types
│
├── extension/                          # Browser extension source code
│   ├── manifest.json                   # Chrome/Firefox Manifest V3 definition
│   ├── popup/                          # Extension popup HTML/CSS/JS
│   ├── background/                     # Background service worker (cookie & auth extraction)
│   └── icons/                          # Extension icons
│
├── go.mod                              # Go module definition
├── go.sum                              # Go module checksums
├── Makefile                            # Build and developer automation tasks
└── README.md                           # Quickstart guide
```

---

## 3. Technology Stack

### 3.1 Backend (Golang)
- **Language**: Go 1.22+
- **HTTP Framework / Router**: `chi` (`github.com/go-chi/chi/v5`) or standard library `net/http` for minimal overhead and readable middleware chaining.
- **HTML Parsing**: `goquery` (`github.com/PuerkitoBio/goquery`) for robust CSS selector querying on `my.kpi.ua`.
- **Telegram Bot SDK**: `gotgbot` (`github.com/PaulSonOfLars/gotgbot/v2`) or `telebot` (`gopkg.in/telebot.v3`) for typed updates, custom keyboards, and webhook/long-polling support.
- **Database / Persistence**: PostgreSQL (or SQLite for lightweight local deployment) using `pgx` or `sqlx`.
- **Caching**: In-memory cache (e.g. `patrickmn/go-cache` or `ristretto`) with optional Redis support for multi-instance deployments.
- **Logging & Observability**: Structured logging via standard `log/slog`.

### 3.2 Browser Extension
- **Platform**: WebExtensions API (Manifest V3), compatible with Google Chrome, Brave, Microsoft Edge, and Mozilla Firefox.
- **Permissions**: `cookies`, `storage`, and `host_permissions: ["https://my.kpi.ua/*", "https://*.kpi.ua/*"]`.
- **UI**: Lightweight HTML5/CSS3/Vanilla JS popup (no heavy frontend framework needed).

---

## 4. Development Principles

1. **Simplicity Over Complexity**: Keep domain structures clean and flat. Avoid unnecessary abstractions.
2. **Explicit Error Handling**: Wrap errors with context (`fmt.Errorf("fetching group lessons: %w", err)`) and notify users gracefully.
3. **Resilience**: The group schedule API (`api.campus.kpi.ua`) is reliable and structured. The personal schedule (`my.kpi.ua`) is server-rendered HTML; scrapers must fail safely with clear error alerts without crashing the server.
