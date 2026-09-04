# Admin API Endpoints Reference

This document details the protected admin endpoints exposed by the Go server under `/api/v1/admin/*`, intended for consumption by the external Admin Web Dashboard.

---

## 1. Authentication & Security

All `/api/v1/admin/*` routes are protected by the `adminTokenMiddleware`.

### Headers Required
- `X-Admin-Secret`: Must match the server's `ADMIN_API_SECRET` (falls back to `INTERNAL_API_TOKEN` if unset).
- `X-Admin-Role`: Passed by the Admin Dashboard backend:
  - `superadmin`: Full privileges (read, write, custom query).
  - `read-write`: Full database privileges (read, write, custom query).
  - `read-only`: Read-only access (`PUT /tables/{table}/row`, `POST /query`, and every issue write — `PATCH /issues/{id}/status`, `PATCH /issues/{id}/thread`, `POST /issues/{id}/comments`, `DELETE /issues/{id}` — return `403 Forbidden`).
- `X-Admin-Email`: Email of the acting admin, forwarded by the dashboard proxy. Recorded as `issues.status_by` and as the `author_label` of admin comments — the telemetry pipeline anonymizes identifiers, so this header is the issue queue's only audit trail.

Protected `/api/v1/admin/*` routes are excluded from the public 20 req/min IP rate limiter since requests proxy through the admin dashboard's server IP.

If `X-Admin-Secret` is missing or invalid, the server responds with:
```json
{
  "success": false,
  "error_code": "ERR_UNAUTHORIZED",
  "message": "missing or invalid X-Admin-Secret header",
  "timestamp": "2026-09-04T10:00:00Z"
}
```

---

## 2. Endpoints

### 2.1 List Database Tables
- **Method**: `GET`
- **Path**: `/api/v1/admin/tables`
- **Access**: `read-only`, `read-write`, `superadmin`
- **Response**: `200 OK`
```json
{
  "tables": [
    { "name": "users", "row_count": 150 },
    { "name": "user_lessons", "row_count": 3200 },
    { "name": "bot_groups", "row_count": 12 }
  ]
}
```

### 2.2 View Table Rows
- **Method**: `GET`
- **Path**: `/api/v1/admin/tables/{table}`
- **Query Parameters**:
  - `limit`: Integer (1–500, default: 50).
  - `offset`: Integer (default: 0).
  - `sort_by`: Column name to sort by.
  - `sort_order`: `asc` or `desc` (default: `asc`).
- **Access**: `read-only`, `read-write`, `superadmin`
- **Response**: `200 OK`
```json
{
  "columns": [
    { "name": "id", "type": "TEXT", "primary_key": true, "not_null": true },
    { "name": "telegram_id", "type": "INTEGER", "primary_key": false, "not_null": true }
  ],
  "rows": [
    { "id": "u1", "telegram_id": 123456789 }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

### 2.3 Update Table Row
- **Method**: `PUT`
- **Path**: `/api/v1/admin/tables/{table}/row`
- **Access**: `read-write`, `superadmin` (`read-only` receives `403 Forbidden`)
- **Validation**: `primary_key_column` must specify a valid primary key column on the table. Tables with composite primary keys cannot be updated in-place via this endpoint.
- **Body**:
```json
{
  "primary_key_column": "id",
  "primary_key_value": "u1",
  "updates": {
    "group_name": "IA-01"
  }
}
```
- **Telemetry**: Reports an anonymous `admin_action` event (`update_row:{table}`) with table metadata to the admin dashboard ingest endpoint.
- **Response**: `200 OK`
```json
{
  "status": "ok"
}
```

### 2.4 Execute Custom SQL Query
- **Method**: `POST`
- **Path**: `/api/v1/admin/query`
- **Access**: `read-write`, `superadmin` (`read-only` receives `403 Forbidden`)
- **Timeout**: Enforces a strict 10-second context timeout to protect SQLite single-connection serial access.
- **Telemetry**: Reports an anonymous `admin_action` event (`custom_query`) with query snippet and rows metadata to the admin dashboard ingest endpoint.
- **Body**:
```json
{
  "query": "SELECT COUNT(*) AS total FROM users WHERE group_id IS NOT NULL"
}
```
- **Response**: `200 OK`
```json
{
  "columns": ["total"],
  "rows": [{ "total": 42 }],
  "rows_affected": 1,
  "duration_ms": 2,
  "truncated": false
}
```
For SELECT/read queries (including `WITH`, `PRAGMA`, `EXPLAIN`, `VALUES`, and `RETURNING` queries), results are capped at a maximum of 1,000 rows to prevent microVM memory spikes. If more rows exist, `truncated` is set to `true` (and the admin dashboard provides pagination controls to query subsequent pages via `LIMIT` and `OFFSET`). For non-SELECT queries (`UPDATE`, `INSERT`, `DELETE`), `columns` and `rows` are omitted and `rows_affected` indicates the number of affected rows.


### 2.5 List Issues
- **Method**: `GET`
- **Path**: `/api/v1/admin/issues`
- **Query Parameters**:
  - `status`: One of `on_review`, `ready`, `in_development`, `implemented`, `duplicate`, `rejected`, `cancelled`. An unknown value returns `400 ERR_INVALID_REQUEST`.
  - `type`: One of `feature`, `bug`, `other`. An unknown value returns `400 ERR_INVALID_REQUEST`.
  - `q`: Substring match against the issue title, body and reporter `@username`. `%` and `_` in the term are matched literally, not as SQL wildcards. Case-insensitivity comes from SQLite's `LIKE`, which folds ASCII only — a Cyrillic term matches case-sensitively.
  - `limit`: Integer (default: 50).
  - `offset`: Integer (default: 0).
- **Access**: `read-only`, `read-write`, `superadmin`
- **Response**: `200 OK`
```json
{
  "issues": [
    {
      "id": "9f1c...",
      "number": 12,
      "author_telegram_id": 123456789,
      "author_username": "student",
      "author_first_name": "Olha",
      "type": "feature",
      "title": "Add calendar export",
      "body": "It would be great to export the week to Google Calendar.",
      "status": "on_review",
      "status_by": "",
      "status_note": "",
      "thread_state": "none",
      "comment_count": 0,
      "created_at": "2026-09-04T10:00:00Z",
      "updated_at": "2026-09-04T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0,
  "status_counts": { "on_review": 1 }
}
```
`status_counts` is the per-status tally the dashboard labels its status tabs with, saving a round trip. It honours the `type` and `q` filters but ignores `status` — the tabs are how a status is picked — so the tab labels always describe the same set of issues the table below them is paging through.

### 2.6 Get Issue with Discussion Thread
- **Method**: `GET`
- **Path**: `/api/v1/admin/issues/{id}`
- **Access**: `read-only`, `read-write`, `superadmin`
- **Errors**: `400 ERR_INVALID_REQUEST` for a malformed UUID, `404 ERR_ISSUE_NOT_FOUND` for an unknown one.
- **Response**: `200 OK`
```json
{
  "issue": { "id": "9f1c...", "number": 12, "thread_state": "open", "comment_count": 2, "...": "as above" },
  "comments": [
    {
      "id": "3a7e...",
      "author_role": "admin",
      "author_label": "admin@example.com",
      "body": "Which calendar app do you use?",
      "created_at": "2026-09-04T10:05:00Z"
    },
    {
      "id": "6b21...",
      "author_role": "user",
      "author_label": "@student",
      "body": "Google Calendar.",
      "created_at": "2026-09-04T10:07:00Z"
    }
  ]
}
```
Comments are ordered oldest first. `author_role` is `admin` or `user`; `author_label` is the admin email or the reporter's `@username` (their first name when they have no username).

Every response carrying an `issue` object reports its real `comment_count`, including the `PATCH` responses of §2.7 and §2.9.

`thread_state` is `none` (no discussion yet), `open` (the reporter can read and reply) or `closed` (the reporter can read but not reply). See §2.9.

### 2.7 Update Issue Status
- **Method**: `PATCH`
- **Path**: `/api/v1/admin/issues/{id}/status`
- **Access**: `read-write`, `superadmin` (`read-only` receives `403 Forbidden`)
- **Body**:
```json
{ "status": "rejected", "note": "Out of scope for this term." }
```
- **Validation**: The status must be one of the seven lifecycle values; anything else returns `400 ERR_INVALID_REQUEST`. `note` is optional, trimmed, and capped at 3000 characters.
- **The optional note** is the way to say *why* — "rejected because…" — in a single message, without opening a discussion. It is delivered to the reporter with the status DM, stored as `issues.status_note`, and shown on their issue screen in the bot so it can be re-read. Sending a change with no `note` clears any previous one, so a stale explanation never outlives the status it explained.
- **Side effects**: Writes `status_by` from `X-Admin-Email` and DMs the reporter over Telegram. Notification failures are logged, never surfaced — the status change is already committed.
- **Telemetry**: Reports an anonymous `admin_action` event (`issue_status:{status}`) with the `from`/`to` statuses.
- **Response**: `200 OK`
```json
{ "issue": { "id": "9f1c...", "status": "rejected", "status_note": "Out of scope for this term.", "...": "" }, "changed": true }
```
Re-sending the status an issue already has is a no-op: the response carries `"changed": false` and the reporter is not notified. Sending a `note` with an unchanged status returns `400 ERR_INVALID_REQUEST` rather than silently dropping it — use the discussion thread to message the reporter without changing the status.

### 2.8 Add Admin Comment
- **Method**: `POST`
- **Path**: `/api/v1/admin/issues/{id}/comments`
- **Access**: `read-write`, `superadmin` (`read-only` receives `403 Forbidden`)
- **Body**:
```json
{ "body": "Which calendar app do you use?" }
```
- **Validation**: The body is trimmed and must be 1–3000 characters; otherwise `400 ERR_INVALID_REQUEST`.
- **Side effects**: The comment is stored with `author_role: "admin"` and `author_label` set from `X-Admin-Email`. The **first** admin comment moves `issues.thread_state` from `none` to `open`, which is what makes the discussion screen visible to the reporter in the bot — threads are admin-initiated by design. The insert and that transition share one transaction, so a comment is never stored behind a thread the bot still considers unstarted (which would leave the reporter a DM button that goes nowhere). The reporter is then DM'd with a button that opens the thread. See [`docs/bot/issues.md`](../bot/issues.md).
- **Closed threads**: posting into a thread whose `thread_state` is `closed` returns `409 ERR_INVALID_REQUEST`. Reopening it (§2.9) is a separate, deliberate act, so a reply never silently resurrects a discussion the reporter was told had ended.
- **Telemetry**: Reports an anonymous `admin_action` event (`issue_comment`).
- **Response**: `201 Created`
```json
{
  "comment": {
    "id": "3a7e...",
    "author_role": "admin",
    "author_label": "admin@example.com",
    "body": "Which calendar app do you use?",
    "created_at": "2026-09-04T10:05:00Z"
  }
}
```


### 2.9 Close or Reopen a Discussion
- **Method**: `PATCH`
- **Path**: `/api/v1/admin/issues/{id}/thread`
- **Access**: `read-write`, `superadmin` (`read-only` receives `403 Forbidden`)
- **Body**:
```json
{ "state": "closed" }
```
- **Validation**: `state` must be `"open"` or `"closed"` — anything else, including `"none"`, returns `400 ERR_INVALID_REQUEST`, since a thread with history is not one that never existed. Either state on a thread that was never started returns `409 ERR_INVALID_REQUEST`: there is nothing to close, and "reopening" an empty thread would hand the reporter a Reply button over a blank transcript. Post a comment (§2.8) to start one.
- **Semantics**: Closing stops the reporter sending new messages; they keep read access to the whole transcript, and the bot replaces their Reply button with a padlock. Reopening restores replies on both sides.
- **Side effects**: DMs the reporter either way, so a Reply button that appears or disappears is never a mystery. Best-effort, as elsewhere.
- **Telemetry**: Reports an anonymous `admin_action` event (`issue_thread:{state}`).
- **Response**: `200 OK`
```json
{ "issue": { "id": "9f1c...", "thread_state": "closed", "...": "" }, "changed": true }
```
Setting the state a thread already has is a no-op: `"changed": false`, and the reporter is not notified.

### 2.10 Delete an Issue
- **Method**: `DELETE`
- **Path**: `/api/v1/admin/issues/{id}`
- **Access**: `read-write`, `superadmin` (`read-only` receives `403 Forbidden`)
- **Semantics**: Removes the issue permanently. Its comments follow via `ON DELETE CASCADE`, and any in-flight reply draft pointing at it is cleared in the same transaction. Deleting an already-deleted issue returns `404 ERR_ISSUE_NOT_FOUND`.
- **The reporter is not notified.** An issue they can no longer open is not something a DM usefully explains, and deletion is normally spam cleanup. It simply disappears from their `/issues` list. Reporters can also delete their own issues from the bot.
- **Telemetry**: Reports an anonymous `admin_action` event (`issue_delete`).
- **Response**: `200 OK`
```json
{ "status": "ok" }
```

---

## 3. Environment Variables (Main Server)

Configure the following variables in `apps/server/.env` (local) or via `fly secrets set` (production):

| Variable | Required | Description | Example / Default |
| :--- | :---: | :--- | :--- |
| `ADMIN_API_SECRET` | No | Secret required in `X-Admin-Secret` header for `/api/v1/admin/*` routes. | Falls back to `INTERNAL_API_TOKEN` |
| `ADMIN_INGEST_URL` | No | Target URL of the Admin Dashboard's telemetry ingest endpoint. If unset, telemetry reporting is disabled. | `https://<admin-app>.vercel.app/api/ingest/action` |
| `ADMIN_INGEST_KEY` | No | Secret key sent in `X-Ingest-Key` to authenticate telemetry payloads. | High-entropy random string matching `ADMIN_INGEST_KEY` in `apps/admin` |

