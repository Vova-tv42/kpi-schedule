# Backend REST API Overview

## 1. Design Principles

The Golang backend API serves as the core coordinator between the Telegram Bot, Browser Extension, and external KPI schedule services.

- **Base URL Prefix**: `/api/v1`
- **Format**: JSON (`Content-Type: application/json; charset=utf-8`)
- **Authentication**:
  - All `/api/v1/*` routes (except `/healthz`) require a shared secret in the `X-Internal-Token` header, checked against `INTERNAL_API_TOKEN`.
  - Browser extension pairing (6-digit one-time code) and the Telegram webhook are **deferred**; this iteration accepts `my.kpi.ua` cookies directly via `POST /api/v1/auth/session` (see `docs/api/auth-endpoints.md`).
- **HTTP Status Codes**:
  - `200 OK`: Request succeeded.
  - `400 Bad Request`: Invalid parameters or invalid pairing code.
  - `401 Unauthorized`: Session expired or invalid bot token.
  - `404 Not Found`: Group or user not found.
  - `500 Internal Server Error`: Unrecoverable scraping or merging failure (accompanied by a structured error code).

> **Note on the bot.** The Telegram bot is not implemented yet. When it is added, it will run **inside this same Go process** (see [`docs/project-repository.md` §4.1](../project-repository.md)) and call the engine/service layer directly rather than issuing HTTP requests to these routes — the `/schedule/*` endpoints exist today purely as a manually-tested, inspectable surface (per the user's "server + API only, tested with curl" scope for this iteration).

---

## 2. API Route Summary

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| **POST** | `/api/v1/auth/session` | Submit `telegram_id`, optional `group_name`, and `my.kpi.ua` cookies directly; probes, resolves the group, stores, and runs the first refresh | `X-Internal-Token` |
| **GET** | `/api/v1/auth/status/{telegramId}` | Check if user's session is active, expired, or not linked | `X-Internal-Token` |
| **DELETE**| `/api/v1/auth/unlink/{telegramId}` | Remove stored cookies and lessons, unbind user | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/today` | Fetch combined schedule for today | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/tomorrow` | Fetch combined schedule for tomorrow | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/week` | Fetch combined 2-week schedule | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/date` | Fetch combined schedule for a specific date | `X-Internal-Token` |
| **POST** | `/api/v1/schedule/refresh` | Force an immediate re-scrape + re-merge | `X-Internal-Token` |
| **GET** | `/api/v1/groups` | Search & list all academic groups | `X-Internal-Token` |
| **GET** | `/api/v1/time/current` | Query current academic week & day | `X-Internal-Token` |
| **POST** | `/api/v1/debug/mykpi/dump` | Capture a raw `my.kpi.ua` HTML fixture for scraper development. Only mounted when `DEBUG_ROUTES=true`; never enable in production | `X-Internal-Token` |

The Telegram bot and its webhook route (`/api/v1/telegram/webhook`) are deferred to a later iteration; see the "Not yet created" note in `docs/project-repository.md` §2.
