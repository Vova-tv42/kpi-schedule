# Auth & Session API Endpoints

> **Correction (post-implementation, architecture decision).** The server no longer accepts
> `my.kpi.ua` cookies at all — see [`docs/architecture/data-storage.md`](../architecture/data-storage.md).
> `POST /api/v1/auth/session` (cookie-based linking) is **removed**. There is no `ACTIVE` /
> `EXPIRED` session concept any more, since the server never authenticates to `my.kpi.ua`
> itself; a user is simply `LINKED` (has at least one pushed schedule) or `NOT_LINKED`. The
> future replacement — the browser extension pushing a parsed schedule — is a schedule-sync
> endpoint documented as **not yet implemented** in
> [`docs/api/schedule-endpoints.md`](schedule-endpoints.md) and
> [`docs/architecture/data-storage.md`](../architecture/data-storage.md) §4, not an auth
> endpoint at all (there's nothing left to authenticate on the server side).

## 1. Check Link Status

- **Endpoint**: `GET /api/v1/auth/status/{telegramId}`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Response**: `200 OK`
```json
{
  "telegram_id": 123456789,
  "status": "LINKED",
  "linked_at": "2026-09-01T08:30:00Z",
  "last_synced_at": "2026-09-01T10:00:00Z",
  "group_id": 5626,
  "group_name": "ІП-54"
}
```
`NOT_LINKED` responses omit `linked_at`/`last_synced_at`/`group_id`/`group_name`:
```json
{
  "telegram_id": 123456789,
  "status": "NOT_LINKED"
}
```
*Status values: `LINKED` (a schedule has been pushed at least once), `NOT_LINKED`.*

---

## 2. Unlink

Removes the user and all stored lessons (`ON DELETE CASCADE`). There are no credentials to
delete any more — the server never held any.

- **Endpoint**: `DELETE /api/v1/auth/unlink/{telegramId}`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Response**: `200 OK`
```json
{
  "success": true,
  "message": "User unlinked and all stored lessons deleted."
}
```
