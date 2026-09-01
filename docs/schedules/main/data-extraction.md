# Personal Schedule Extraction Strategy (my.kpi.ua)

> **Status: verified against live data.** The original version of this document assumed
> `my.kpi.ua` renders a static HTML schedule table (`.odd-week`/`.even-week`/`.c_cell`
> selectors), inferred from CSS files without ever seeing a real logged-in page. That
> assumption was **wrong**. The real mechanism, documented below, was reverse-engineered
> from a real fixture capture and confirmed with `curl` against the live endpoint (read-only
> `GET` requests only).

## 1. The calendar page is a FullCalendar.js shell, not an HTML table

- **Shell page**: `GET https://my.kpi.ua/room/student/calendar` — requires `Cookie:
  PHPSESSID=...; _identity=...` and a browser-like `User-Agent`.
- The response HTML does **not** contain the lesson data. It embeds a FullCalendar.js
  widget configured with an inline `<script>` block that names a JSON events source:
  ```
  "events":"/calendar/studevents?id=33101"
  ```
  `33101` is the authenticated student's internal ID — it is only known after parsing this
  page, so fetching a schedule is a **two-step process**: fetch the shell page, extract the
  events URL, then fetch the events JSON. `apps/server/internal/mykpi/client.go`'s
  `FetchStudentEventsRange` implements exactly this sequence.

## 2. The events endpoint requires an explicit date range

`GET https://my.kpi.ua/calendar/studevents?id=<studentId>&start=YYYY-MM-DD&end=YYYY-MM-DD`

This is standard FullCalendar behavior for a string-URL event source: the widget always
appends the visible range as `start`/`end` query params. **Without them the endpoint
silently returns `[]`** — confirmed directly against the live endpoint. There is no
documented upper bound on the range; the server requests a fixed window on every refresh
(`fetchWindowPast`/`fetchWindowFuture` in `apps/server/internal/api/service.go`, currently
14 days back / 120 days forward) rather than the widget's own visible-month range.

## 3. The payload is already exact-dated, fully personalized data

This is the single biggest correction to the original plan. The response is a flat JSON
array of concrete lesson **occurrences** — not a recurring week-1/week-2 pattern:

```json
[
  {
    "id": 1019849,
    "title": "Технології DevOps",
    "start": "2026-09-19T08:30:00",
    "end": "2026-09-19T10:05:00",
    "description": "<i><span title=\"...\">Колумбет В. П.</span></i>",
    "descriptionRAW": "Викладачі: Колумбет В. П.",
    "extendedProps": {
      "type": "lec",
      "teachers": "<i>...</i>",
      "longTitle": "<em>Технології DevOps <sup>lec</sup></em><br/>...",
      "location": "<span title=\"Онлайн\">...</span>",
      "locationRAW": ", URL: Не вказано",
      "locationPDF": "lec., Онлайн Zoom",
      "locationURL": null,
      "groups": "ТВ-41, ТВ-42, ТВ-43",
      "timegrid": 1,
      "modularity": 1
    }
  }
]
```

Each event is already filtered to exactly what this student attends — every elective and
subgroup ambiguity is already resolved server-side by my.kpi.ua. There is no personal-side
concept of "which pattern occurs on which dates" to resolve; `start`'s calendar date is the
lesson's actual, authoritative occurrence date. This is why `model.Lesson.Date` (not
Week/Day/Slot) is the schema's primary key component — see
[`docs/architecture/data-storage.md`](../../architecture/data-storage.md). Week/Day/Slot are
still stored, but only as *derived* display/matching fields, computed at refresh time via
`engine.WeekAt`/`engine.ISODay` against the Campus API's current-week anchor.

## 4. Field mapping (`apps/server/internal/mykpi/parser.go`)

| JSON field | Parsed into | Notes |
|---|---|---|
| `start` (`YYYY-MM-DDTHH:MM:SS`) | `Date` + `StartTime` | date component is authoritative; `HH:MM:SS` kept separately |
| `end` | `EndTime` | `HH:MM:SS` |
| `title` | `Subject` | used as-is; normalized separately for Campus matching via `engine.NormalizeSubject` |
| `extendedProps.type` | `Tag` | mapped through `normalizeMyKPITag`: `"lec"→"lec"`, **`"prc"→"prac"`** (my.kpi.ua's own code differs from Campus's `"prac"`), `"lab"→"lab"`, anything else → `""` |
| `descriptionRAW` | `TeacherRaw` | strips the `"Викладачі: "` label prefix if present |
| `extendedProps.locationPDF` | `LocationRaw` | plain-text fallback field (the `location`/`description` fields are HTML fragments and deliberately not parsed) |

No `"lab"`-tagged event has been observed yet — the student used to capture fixtures had a
fully remote semester. If a future capture surfaces a different `type` code or an in-person
`locationPDF` shape, extend `normalizeMyKPITag` and re-verify against a fresh fixture rather
than guessing.

## 5. Enrichment vs. this source

The Campus API (`api.campus.kpi.ua`) is still needed for lecturer IDs/map URIs and as the
matching target for week/day parity — see
[`docs/architecture/merging-engine.md`](../../architecture/merging-engine.md). But it is no
longer used to re-derive *occurrence dates*: since the personal source is already
exact-dated, that staleness-guard mechanism from the original plan is gone.

## 6. Fixture capture workflow (for future re-verification)

1. Start the server with `DEBUG_ROUTES=true` (the `.env.example` default).
2. Log into `my.kpi.ua` in a browser, copy `PHPSESSID` and `_identity` from DevTools →
   Application → Cookies.
3. `curl -X POST -H "X-Internal-Token: $INTERNAL_API_TOKEN" -H 'Content-Type: application/json' \
       -d '{"cookies":{"PHPSESSID":"...","_identity":"..."}}' \
       localhost:8080/api/v1/debug/mykpi/dump`
4. The calendar shell HTML and the events JSON (fetched over the server's default fetch
   window) are saved to `apps/server/internal/mykpi/testdata/calendar-<timestamp>.html` and
   `events-<timestamp>.json`. Both are gitignored (`*.html` and raw dumps carry a real
   student's PII) — scrub and trim before ever promoting one to a committed golden fixture
   like `testdata/events-golden.json`.
