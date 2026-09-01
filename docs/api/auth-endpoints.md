# Auth & Session API Endpoints

> **Status note.** The pairing-code handshake (`/auth/pair-code` + browser extension) is
> **deferred**. This iteration accepts `my.kpi.ua` cookies directly in one call, per the
> user's "server + API only, cookies posted directly, tested with curl" scope. The schema and
> encryption underneath are unchanged, so the pairing flow can be layered on top later without
> a data-model change — see `docs/project-repository.md` §2 "Not yet created".

## 1. Link a Session

- **Endpoint**: `POST /api/v1/auth/session`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Request Body**:
```json
{
  "telegram_id": 123456789,
  "group_name": "ІП-54",
  "cookies": {
    "PHPSESSID": "eb659e5b8a5f5a4ea1d4f20ef1443af9",
    "_identity": "39233868b6449f77b496598a9824806e3adf855c...",
    "language": "uk"
  },
  "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."
}
```

**Behavior**:
1. If `group_name` is given, it is resolved to a Campus `groupId` via the group catalog
   (`GET /group/all`, cached 24h) — `404`-style failure if not found.
2. The user row is created or updated (`telegram_id` is the external key).
3. The cookies are probed against `https://my.kpi.ua/room/student/calendar` **before** being
   stored — an invalid cookie set is never persisted as "active".
4. On success, the cookies are AES-256-GCM encrypted (see `docs/architecture/data-storage.md`
   §3) and stored, and a first schedule refresh runs immediately.

- **Response (`200 OK`)**:
```json
{
  "success": true,
  "message": "Session successfully validated and linked.",
  "telegram_id": 123456789,
  "group_name": "ІП-54",
  "enrichment_status": "full"
}
```
- **Error Response (`401 Unauthorized`)**:
```json
{
  "success": false,
  "error_code": "ERR_INVALID_SESSION_COOKIES",
  "message": "Could not authenticate to my.kpi.ua with the provided cookies."
}
```

---

## 2. Check Session Status

- **Endpoint**: `GET /api/v1/auth/status/{telegramId}`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Response**: `200 OK`
```json
{
  "telegram_id": 123456789,
  "status": "ACTIVE",
  "linked_at": "2026-09-01T08:30:00Z",
  "last_synced_at": "2026-09-01T10:00:00Z",
  "group_id": 5626,
  "group_name": "ІП-54"
}
```
*Status values: `ACTIVE`, `EXPIRED`, `NOT_LINKED`.*

---

## 3. Unlink Session

Removes the user's cookies and all stored lessons (`ON DELETE CASCADE`).

- **Endpoint**: `DELETE /api/v1/auth/unlink/{telegramId}`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Response**: `200 OK`
```json
{
  "success": true,
  "message": "Session unlinked and credentials deleted."
}
```
