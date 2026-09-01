# Schedule API Endpoints

All schedule endpoints return the unified, deduplicated, and enriched schedule for the requested student.

---

## 1. Get Schedule for Today

- **Endpoint**: `GET /api/v1/schedule/today`
- **Headers**: `X-Internal-Token: <SECRET_BOT_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required): The Telegram user ID.
  - `force_refresh` (bool, optional): Skip cache and re-scrape `my.kpi.ua`. Default `false`.

### Response (`200 OK`)
```json
{
  "date": "2026-09-01",
  "week": 1,
  "day_name": "Вівторок",
  "day_short": "Вв",
  "is_day_off": false,
  "enrichment_status": "full",
  "lessons": [
    {
      "slot": 1,
      "time": "08:30:00",
      "name": "Процеси розробки вбудованого програмного забезпечення",
      "type": "Лекція",
      "tag": "lec",
      "lecturer": {
        "id": "ce02d4b6d1aceeea96a562c10923d590607df6182b4a3405ad10be85b6354787",
        "name": "Гуменний Дмитро Олександрович"
      },
      "location": {
        "title": "18-402",
        "uri": "https://kpi.ua/k-18"
      },
      "is_selective": false
    },
    {
      "slot": 2,
      "time": "10:25:00",
      "name": "Технології DevOps",
      "type": "Практика",
      "tag": "prac",
      "lecturer": {
        "id": "a3513549948fbfecfa1ff0300bf3339f9b1ebff77c9fc831b3c83236a4814d61",
        "name": "Колумбет Вадим Петрович"
      },
      "location": {
        "title": "5-508",
        "uri": "https://kpi.ua/k-5"
      },
      "is_selective": true
    }
  ]
}
```

---

## 2. Get Schedule for Tomorrow

- **Endpoint**: `GET /api/v1/schedule/tomorrow`
- **Headers**: `X-Internal-Token: <SECRET_BOT_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required)

*(Response structure is identical to `/api/v1/schedule/today`)*

---

## 3. Get Weekly Schedule

- **Endpoint**: `GET /api/v1/schedule/week`
- **Headers**: `X-Internal-Token: <SECRET_BOT_TOKEN>`
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
- **Headers**: `X-Internal-Token: <SECRET_BOT_TOKEN>`
- **Query Parameters**:
  - `telegram_id` (int64, required)
  - `date` (string, required): Format `YYYY-MM-DD` (e.g. `2026-09-15`).

---

## 5. Group Search Directory

- **Endpoint**: `GET /api/v1/groups`
- **Query Parameters**:
  - `query` (string, optional): Search keyword (e.g. `ІП-21` or `ФІОТ`).
- **Response**: List of matching groups with ID, name, and faculty.
