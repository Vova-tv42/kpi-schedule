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
  - `read-only`: Read-only access (`PUT /tables/{table}/row` and `POST /query` return `403 Forbidden`).

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
For SELECT/read queries, results are capped at a maximum of 1,000 rows to prevent microVM memory spikes. If more rows exist, `truncated` is set to `true` (and the admin dashboard provides pagination controls to query subsequent pages via `LIMIT` and `OFFSET`). For non-SELECT queries (`UPDATE`, `INSERT`, `DELETE`), `columns` and `rows` are omitted and `rows_affected` indicates the number of affected rows.

---

## 3. Environment Variables (Main Server)

Configure the following variables in `apps/server/.env` (local) or via `fly secrets set` (production):

| Variable | Required | Description | Example / Default |
| :--- | :---: | :--- | :--- |
| `ADMIN_API_SECRET` | No | Secret required in `X-Admin-Secret` header for `/api/v1/admin/*` routes. | Falls back to `INTERNAL_API_TOKEN` |
| `ADMIN_INGEST_URL` | No | Target URL of the Admin Dashboard's telemetry ingest endpoint. If unset, telemetry reporting is disabled. | `https://<admin-app>.vercel.app/api/ingest/action` |
| `ADMIN_INGEST_KEY` | No | Secret key sent in `X-Ingest-Key` to authenticate telemetry payloads. | High-entropy random string matching `ADMIN_INGEST_KEY` in `apps/admin` |

