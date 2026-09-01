# Schedule API Endpoints

All schedule endpoints return the unified, deduplicated, and enriched schedule for the requested student.

> **Status note.** Responses now also carry `stale` (bool), `session_status` (`active` /
> `expired`), and `enrichment_status` (`full` / `degraded` / `none`) — see the no-cron refresh
> policy in `docs/architecture/data-storage.md` §4. A lesson's `is_selective` field from the
> original draft below has been dropped: it isn't honestly derivable from either source (it
> would require knowing every elective the group offers, not just what the student attends).
> Each lesson instead carries `enriched` (bool): whether it was matched against the group
> schedule for lecturer/room enrichment, or is a raw, un-enriched personal entry.
>
> **Correction (post-implementation).** Each lesson now carries its own `date` (my.kpi.ua's
> personal feed turned out to return exact-dated occurrences, not a recurring pattern — see
> `docs/schedules/main/data-extraction.md`), plus `end_time`. The free-form `type` field
> (e.g. `"Лекція"`) was dropped in favor of `tag` alone (`"lec"`/`"prac"`/`"lab"`) — it was
> redundant. `teacher_raw`/`location_raw` are new: my.kpi.ua's own plain-text values, included
> as a fallback alongside the resolved `lecturer`/`location` objects (present only when
> `enriched: true`).

---

## 1. Get Schedule for Today

- **Endpoint**: `GET /api/v1/schedule/today`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required): The Telegram user ID.
  - `force_refresh` (bool, optional): Force an inline re-scrape of `my.kpi.ua`. Default `false`.

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
  "session_status": "active",
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

---

## 5. Force Refresh

- **Endpoint**: `POST /api/v1/schedule/refresh`
- **Headers**: `X-Internal-Token: <INTERNAL_API_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required)
- **Response**: `200 OK` — `{"success": true, "enrichment_status": "full"}`

---

## 6. Group Search Directory

- **Endpoint**: `GET /api/v1/groups`
- **Query Parameters**:
  - `query` (string, optional): Search keyword (e.g. `ІП-54` or `ФІОТ`).
- **Response**: List of matching groups with ID, name, and faculty.
