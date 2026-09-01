# Personal Schedule Extraction Strategy (my.kpi.ua)

> **Status: verified against live data.** The original version of this document assumed
> `my.kpi.ua` renders a static HTML schedule table (`.odd-week`/`.even-week`/`.c_cell`
> selectors), inferred from CSS files without ever seeing a real logged-in page. That
> assumption was **wrong**. The real mechanism, documented below, was reverse-engineered
> from a real fixture capture and confirmed with `curl` against the live endpoint (read-only
> `GET` requests only).
>
> **Correction (post-implementation, architecture decision).** This fetch used to be
> performed server-side, by `apps/server/internal/mykpi/`. That package is now **deleted** —
> the server never talks to `my.kpi.ua` any more. The mechanism documented below is still
> entirely accurate; it just describes what the (not-yet-built) browser extension must now
> replicate **client-side**, in the student's own browser session, before pushing the parsed
> result to the backend. See [`docs/architecture/data-storage.md`](../../architecture/data-storage.md)
> and [`docs/extension/browser-extension-design.md`](../../extension/browser-extension-design.md).

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
  events URL, then fetch the events JSON. The extension's `fetch-schedule.ts` (see
  [`docs/extension/browser-extension-design.md`](../../extension/browser-extension-design.md))
  must implement exactly this sequence.

## 2. The events endpoint requires an explicit date range

`GET https://my.kpi.ua/calendar/studevents?id=<studentId>&start=YYYY-MM-DD&end=YYYY-MM-DD`

This is standard FullCalendar behavior for a string-URL event source: the widget always
appends the visible range as `start`/`end` query params. **Without them the endpoint
silently returns `[]`** — confirmed directly against the live endpoint. There is no
documented upper bound on the range; the extension should request a fixed window on every
sync (e.g. 14 days back / 120 days forward, matching the old server-side default) rather
than the widget's own visible-month range. The exact window is an extension implementation
detail, not yet finalized.

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
still stored, but only as *derived* display/matching fields, computed at merge time via
`engine.WeekAt`/`engine.ISODay` against the Campus API's current-week anchor.

## 4. Field mapping (target: the extension's `parse-schedule.ts`)

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

There is no server-side debug-dump endpoint any more (`internal/mykpi` and its
`/api/v1/debug/mykpi/dump` route are both deleted). Capturing a fresh fixture is now a
browser-only exercise:

1. Log into `my.kpi.ua` in a browser.
2. Open DevTools → Network, visit `https://my.kpi.ua/room/student/calendar`, and save the
   response body (the shell page HTML) if the shell's inline-script format needs
   re-verifying.
3. In the same Network tab, find the `GET /calendar/studevents?id=...&start=...&end=...`
   request that the page fires automatically, and save its JSON response.
4. Scrub any real student's PII (names, IDs) before promoting a capture to a committed
   fixture. There is no scraper/parser package (and no `testdata/` directory) in the repo
   any more to put one in — a future extension-side test suite would need its own.
