# Lesson Notifications & Scheduled Cron Architecture

This document details the architecture and operational guide for automated lesson notifications (10 minutes before and at the start of classes) for personal users and group chats, their interaction with Fly.io scale-to-zero microVMs, idempotency guarantees, and setup on external cron services (e.g. `cron-job.org`).

---

## 1. Overview & Notification Strategy

Students and academic group chats can receive automated Telegram alerts for scheduled classes:
- **Pre-Lesson Alert (15 to 5 minutes before start)**:
  ```html
  <blockquote>🔔 Пара почнеться через 10 хвилин</blockquote>

  <code>08:30</code>  Практичний курс іноземної мови. Частина 2 <i>(прак.)</i>
  ```
  *(Dynamic `N` minutes declension: `хвилину`, `хвилини`, `хвилин` based on moment of delivery).*
- **Start Alert (5 minutes before to 5 minutes after start)**:
  ```html
  <blockquote>🔔 Почалась пара</blockquote>

  <code>08:30</code>  Практичний курс іноземної мови. Частина 2 <i>(прак.)</i>
  ```
  *(Displays the exact scheduled lesson start time regardless of the exact minute the cron arrived).*
- **Inline Conference Button**:
  If the lesson has a configured conference link (Zoom, Google Meet, Teams, etc.), an inline button with a direct link is attached:
  `[ 🤙 Практичний курс іноземної мови. Частина 2 (Zoom) ↗ ]`
  If there is no URL, no button is attached.

### Personal vs. Group Notifications
1. **Personal Students**:
   - Sent directly to the student's Telegram DM (`users.telegram_id`).
   - Sourced from their merged personalized schedule in `user_lessons` (respecting individual subgroups, electives, and custom URLs in `user_lesson_urls`).
   - Configurable in DM via `/settings`.
2. **Academic Group Chats**:
   - Sent to the linked group chat (`bot_groups.telegram_chat_id`).
   - Sourced from the official Campus timetable (`api.campus.kpi.ua`) for the configured academic group, enriched with group conference URLs (`bot_group_lesson_urls`).
   - Configurable by group administrators via `/group` in DM (`[ 🔔 Сповіщення: Увімкнено / 🔕 Вимкнено ]`).

---

## 2. Fly.io Scale-to-Zero Compatibility

Because the server VM shuts down after 15 minutes of inactivity (`IDLE_TIMEOUT=15m`), in-process timers (`time.Ticker`) cannot fire while the machine is in the `stopped` state.

```
┌──────────────────────────────────────┐
│       External Cron Service          │
│          (cron-job.org)              │
└──────────────────┬───────────────────┘
                   │ HTTP POST /api/v1/cron/lesson-alerts
                   │ Header: Authorization: Bearer <CRON_SECRET>
                   ▼
┌──────────────────────────────────────┐
│              Fly Proxy               │
│  - If stopped: boots VM in < 500ms   │
│  - Forwards request to Go server     │
└──────────────────┬───────────────────┘
                   ▼
┌──────────────────────────────────────┐
│       internal/alerts.Dispatcher     │
│  1. Resolves Europe/Kyiv time        │
│  2. Matches (5..15m] & [-5..5m]      │
│  3. Verifies sent_lesson_alerts      │
│  4. Sends Telegram messages          │
│  5. Records sent alert record        │
└──────────────────┬───────────────────┘
                   ▼
┌──────────────────────────────────────┐
│             HTTP 200 OK              │
│  - Server returns summary JSON       │
│  - If idle for 15m, VM stops         │
└──────────────────────────────────────┘
```

### Why One Daily Wake-Up Is Not Enough
If a service triggers the server only once a day (e.g. at 07:00 AM), the server would wake up, finish in < 1 second, and go to sleep at 07:15 AM. When classes start at 08:30 (and 10:25, 12:20, 14:15, etc.), the machine would be stopped and unable to execute any code. Therefore, an external webhook trigger must ping the server at or around the standard KPI lesson times.

---

## 3. Webhook Endpoint Specification

- **Path**: `POST /api/v1/cron/lesson-alerts` (also supports `GET`)
- **Authentication**:
  - `Authorization: Bearer <CRON_SECRET>` header, or
  - `X-Cron-Secret: <CRON_SECRET>` header, or
  - `?secret=<CRON_SECRET>` query parameter.
  *(If `CRON_SECRET` environment variable is not explicitly defined, it defaults to `INTERNAL_API_TOKEN`).*
- **Response**: `200 OK`
```json
{
  "success": true,
  "personal_alerts_sent": 3,
  "group_alerts_sent": 1,
  "dispatched_at": "2026-09-04T07:20:01.123456Z"
}
```

---

## 4. Idempotency & Deduplication

To prevent duplicate messages if the external cron retries or fires multiple times within a window:
- Dispatched alerts are logged in the `sent_lesson_alerts` SQLite table:
  ```sql
  CREATE TABLE sent_lesson_alerts (
      id             TEXT PRIMARY KEY,
      recipient_type TEXT NOT NULL CHECK (recipient_type IN ('user', 'group')),
      recipient_id   TEXT NOT NULL,
      lesson_date    TEXT NOT NULL,
      lesson_time    TEXT NOT NULL,
      alert_type     TEXT NOT NULL CHECK (alert_type IN ('before_10m', 'at_start')),
      sent_at        TIMESTAMP NOT NULL,
      UNIQUE (recipient_type, recipient_id, lesson_date, lesson_time, alert_type)
  );
  ```
- Before sending any message, `HasAlertBeenSent` checks this table.
- Records older than 7 days are pruned automatically during dispatch to keep the table compact.

---

## 5. Local Development Fallback

In local development or persistent hosting where `IDLE_TIMEOUT <= 0` (idle shutdown disabled), `cmd/server/main.go` automatically spawns an in-process background ticker that runs `alertDispatcher.Dispatch(...)` every 1 minute. No external cron service is required for local offline testing.

---

## 6. Step-by-Step Setup on cron-job.org

[cron-job.org](https://cron-job.org) is a free, reliable web cron service that supports custom schedules, headers, and timezones.

### Step 1: Create an Account
1. Sign up at [https://cron-job.org](https://cron-job.org).
2. Go to the **Cronjobs** dashboard.

### Step 2: Create the Cron Job
1. Click **Create Cronjob**.
2. **Title**: `KPI Schedule Lesson Alerts`
3. **URL**: `https://<your-fly-app>.fly.dev/api/v1/cron/lesson-alerts`
4. **Execution Schedule**:
   - **Timezone**: Select `Europe/Kyiv` (or `Europe/Kiev`).
   - **Option A (Exact KPI Alert Times)**:
     KPI standard lesson start bells are: `08:30, 10:25, 12:20, 14:15, 16:10, 18:30`.
     Set cron schedule for 10m before and at start:
     Minutes: `15, 20, 25, 30`
     Hours: `8, 10, 12, 14, 16, 18`
     Days of week: `Monday - Saturday` (uncheck Sunday).
   - **Option B (Every 10 minutes during classes)**:
     Schedule: `Every 10 minutes`
     Between `08:00` and `19:00`, Monday through Saturday.
5. **Request Method & Headers**:
   - Method: `POST` (or `GET`).
   - Headers: Add header:
     - Name: `Authorization`
     - Value: `Bearer <YOUR_INTERNAL_API_TOKEN_OR_CRON_SECRET>`
6. **Save**: Click **Create**.
