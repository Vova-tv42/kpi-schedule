# Schedule API Endpoints

All schedule endpoints return the unified, deduplicated, and enriched schedule for the requested student.

> **Correction (post-implementation, architecture decision).** The server no longer fetches
> anything on a schedule read — see [`docs/architecture/data-storage.md`](../architecture/data-storage.md).
> The `force_refresh` query parameter and `POST /api/v1/schedule/refresh` are **removed**:
> there is nothing left for the server to refresh, since it has no credentials to fetch with.
> Reads are purely passive lookups of what the (not-yet-built) browser extension has already
> pushed; `401 ERR_AUTH_REQUIRED` is returned if nothing has ever been pushed for the user. The
> `session_status` field is gone along with the session concept — `stale` is the only
> freshness signal left, and it now means "the extension hasn't pushed in over 14 days," not
> "your session expired."
>
> **Correction (post-implementation).** Each lesson now carries its own `date` (my.kpi.ua's
> personal feed turned out to return exact-dated occurrences, not a recurring pattern — see
> `docs/schedules/main/data-extraction.md`), plus `end_time`. The free-form `type` field
> (e.g. `"Лекція"`) was dropped in favor of `tag` alone (`"lec"`/`"prac"`/`"lab"`) — it was
> redundant. `teacher_raw`/`location_raw` are new: my.kpi.ua's own plain-text values, included
> as a fallback alongside the resolved `lecturer`/`location` objects (present only when
> `enriched: true`). The `is_selective` field from the original draft was dropped: it isn't
> honestly derivable from either source.

---

## 1. Get Schedule for Today

- **Endpoint**: `GET /api/v1/schedule/today`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required): The Telegram user ID.

### Response (`200 OK`)
```json
{
  "date": "2026-09-01",
  "week": 1,
  "day_name": "Вівторок",
  "day_short": "Вв",
  "is_day_off": false,
  "enrichment_status": "full",
  "stale": false,
  "lessons": [
    {
      "date": "2026-09-01",
      "slot": 1,
      "time": "08:30:00",
      "end_time": "10:05:00",
      "name": "Процеси розробки вбудованого програмного забезпечення",
      "tag": "lec",
      "teacher_raw": "Гуменний Д. О.",
      "location_raw": "lec., ауд. 402",
      "lecturer": {
        "id": "ce02d4b6d1aceeea96a562c10923d590607df6182b4a3405ad10be85b6354787",
        "name": "Гуменний Дмитро Олександрович"
      },
      "location": {
        "title": "18-402",
        "uri": "https://kpi.ua/k-18"
      },
      "enriched": true
    },
    {
      "date": "2026-09-01",
      "slot": 2,
      "time": "10:25:00",
      "end_time": "12:00:00",
      "name": "Технології DevOps",
      "tag": "prac",
      "teacher_raw": "Колумбет В. П.",
      "location_raw": "prc., Онлайн Zoom",
      "lecturer": {
        "id": "a3513549948fbfecfa1ff0300bf3339f9b1ebff77c9fc831b3c83236a4814d61",
        "name": "Колумбет Вадим Петрович"
      },
      "location": {
        "title": "5-508",
        "uri": "https://kpi.ua/k-5"
      },
      "enriched": true
    }
  ]
}
```

### Error Response (`401 Unauthorized`)
```json
{
  "success": false,
  "error_code": "ERR_AUTH_REQUIRED",
  "message": "no schedule data stored yet; sync the browser extension first",
  "timestamp": "2026-09-01T10:15:00Z"
}
```
Returned whenever nothing has ever been pushed for this `telegram_id`.

---

## 2. Get Schedule for Tomorrow

- **Endpoint**: `GET /api/v1/schedule/tomorrow`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required)

*(Response structure is identical to `/api/v1/schedule/today`)*

---

## 3. Get Weekly Schedule

- **Endpoint**: `GET /api/v1/schedule/week`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required)
  - `week` (int, optional): `1` (Numerator), `2` (Denominator), or omit for both weeks.

### Response (`200 OK`)
```json
{
  "telegram_id": 123456789,
  "current_week": 1,
  "enrichment_status": "full",
  "stale": false,
  "weeks": [
    {
      "week_number": 1,
      "week_name": "Перший тиждень (Чисельник)",
      "days": [
        {
          "day": "Пн",
          "day_name": "Понеділок",
          "lessons": [...]
        },
        {
          "day": "Вв",
          "day_name": "Вівторок",
          "lessons": [...]
        }
      ]
    },
    {
      "week_number": 2,
      "week_name": "Другий тиждень (Знаменник)",
      "days": [...]
    }
  ]
}
```

---

## 4. Get Schedule for a Specific Date

- **Endpoint**: `GET /api/v1/schedule/date`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required)
  - `date` (string, required): Format `YYYY-MM-DD` (e.g. `2026-09-15`).

*(Response structure is identical to `/api/v1/schedule/today`)*

---

## 5. Group Search Directory

- **Endpoint**: `GET /api/v1/groups`
- **Query Parameters**:
  - `query` (string, optional): Search keyword (e.g. `ІП-54` or `ФІОТ`).
- **Response**: List of matching groups with ID, name, and faculty.
