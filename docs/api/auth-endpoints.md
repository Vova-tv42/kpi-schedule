# Auth & Pairing API Endpoints

## 1. Generate Pairing Code

Initiated by the Telegram Bot when a user issues the `/link` command.

- **Endpoint**: `POST /api/v1/auth/pair-code`
- **Headers**: `X-Internal-Token: <SECRET_BOT_TOKEN>`
- **Request Body**:
```json
{
  "telegram_id": 123456789,
  "username": "student_handle"
}
```
- **Response**: `200 OK`
```json
{
  "success": true,
  "pair_code": "742918",
  "expires_at": "2026-09-01T10:25:00Z",
  "ttl_seconds": 600
}
```

---

## 2. Sync Session from Browser Extension

Called directly by the Browser Extension popup when the user submits their pairing code.

- **Endpoint**: `POST /api/v1/auth/sync-session`
- **Request Body**:
```json
{
  "pair_code": "742918",
  "cookies": {
    "PHPSESSID": "eb659e5b8a5f5a4ea1d4f20ef1443af9",
    "_identity": "39233868b6449f77b496598a9824806e3adf855c...",
    "language": "uk"
  },
  "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."
}
```
- **Response (`200 OK`)**:
```json
{
  "success": true,
  "message": "Session successfully validated and linked.",
  "student_name": "Іваненко Іван Іванович",
  "group_name": "ІП-21"
}
```
- **Error Response (`400 Bad Request`)**:
```json
{
  "success": false,
  "error_code": "ERR_INVALID_OR_EXPIRED_PAIR_CODE",
  "message": "The pair code is invalid or has expired."
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

## 3. Check Session Status

Used by the Telegram Bot to display account status in `/settings`.

- **Endpoint**: `GET /api/v1/auth/status/{telegramId}`
- **Headers**: `X-Internal-Token: <SECRET_BOT_TOKEN>`
- **Response**: `200 OK`
```json
{
  "telegram_id": 123456789,
  "status": "ACTIVE",
  "linked_at": "2026-09-01T08:30:00Z",
  "last_synced_at": "2026-09-01T10:00:00Z",
  "group_id": 4402,
  "group_name": "ІП-21"
}
```
*Status values: `ACTIVE`, `EXPIRED`, `NOT_LINKED`.*

---

## 4. Unlink Session

Removes stored cookies and reverts the user to group-only schedule mode.

- **Endpoint**: `DELETE /api/v1/auth/unlink/{telegramId}`
- **Headers**: `X-Internal-Token: <SECRET_BOT_TOKEN>`
- **Response**: `200 OK`
```json
{
  "success": true,
  "message": "Session unlinked and credentials deleted."
}
```
