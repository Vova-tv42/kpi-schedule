# Backend REST API Overview

> **Correction (post-implementation, architecture decision).** The server no longer accepts
> `my.kpi.ua` cookies — see [`docs/architecture/data-storage.md`](../architecture/data-storage.md).
> `POST /api/v1/auth/session` and `POST /api/v1/schedule/refresh` are **removed**, along with
> the debug-only `POST /api/v1/debug/mykpi/dump` (the whole `internal/mykpi` scraper package
> it exercised is gone). The route table below reflects what actually exists today. The
> browser extension's future schedule-sync endpoint (something like `POST
> /api/v1/schedule/sync`) is **not implemented yet** — see
> [`docs/architecture/data-storage.md`](../architecture/data-storage.md) §4.

## 1. Design Principles

The Golang backend API serves as the core coordinator between the Telegram Bot, Browser Extension, and external KPI schedule services.

- **Base URL Prefix**: `/api/v1`
- **Format**: JSON (`Content-Type: application/json; charset=utf-8`)
- **Authentication**:
  - All `/api/v1/*` routes (except `/healthz`) require a shared secret in the `X-Internal-Token` header, checked against `INTERNAL_API_TOKEN`.
  - The server never authenticates to `my.kpi.ua` itself and holds no student credentials — that happens entirely client-side in the (not-yet-built) browser extension, in the student's own browser session.
- **HTTP Status Codes**:
  - `200 OK`: Request succeeded.
  - `400 Bad Request`: Invalid parameters.
  - `401 Unauthorized`: Missing/invalid `X-Internal-Token`, or no schedule data stored yet for the user (`ERR_AUTH_REQUIRED`).
  - `404 Not Found`: Group or user not found.
  - `429 Too Many Requests`: More than 20 requests/minute from this client IP (`ERR_RATE_LIMITED`) — see [`docs/architecture/error-handling-resilience.md`](../architecture/error-handling-resilience.md) §5.
  - `500 Internal Server Error`: Unrecoverable merging failure (accompanied by a structured error code).

> **Note on the bot.** The Telegram bot is not implemented yet. When it is added, it will run **inside this same Go process** (see [`docs/project-repository.md` §4.1](../project-repository.md)) and call the engine/service layer directly rather than issuing HTTP requests to these routes — the `/schedule/*` endpoints exist today purely as a manually-tested, inspectable surface.

---

## 2. API Route Summary

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| **GET** | `/api/v1/auth/status/{telegramId}` | Check if user is `LINKED` (has a pushed schedule) or `NOT_LINKED` | `X-Internal-Token` |
| **DELETE**| `/api/v1/auth/unlink/{telegramId}` | Delete the user and all stored lessons | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/today` | Read stored combined schedule for today | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/tomorrow` | Read stored combined schedule for tomorrow | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/week` | Read stored combined 2-week schedule | `X-Internal-Token` |
| **GET** | `/api/v1/schedule/date` | Read stored combined schedule for a specific date | `X-Internal-Token` |
| **GET** | `/api/v1/groups` | Search & list all academic groups | `X-Internal-Token` |
| **GET** | `/api/v1/time/current` | Query current academic week & day | `X-Internal-Token` |

All `/schedule/*` reads are passive lookups of whatever was last stored — there is no
inline fetch and no `force_refresh` parameter any more (see
[`docs/api/schedule-endpoints.md`](schedule-endpoints.md)).

The Telegram bot's webhook route, and the browser extension's schedule-sync ingestion
route, are both deferred to a later iteration; see the "Not yet created" note in
`docs/project-repository.md` §2.
