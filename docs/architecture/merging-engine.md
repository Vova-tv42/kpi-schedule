# Schedule Merging & Enrichment Engine

> **Correction (post-implementation).** This document originally assumed the personal
> schedule (`my.kpi.ua`) was a recurring week-1/week-2 matrix, like the group schedule, and
> that occurrence dates needed resolving from the Campus API's `dates[]` on every read. Real
> testing showed the personal feed already returns concrete, exact-dated lesson
> occurrences — see [`docs/schedules/main/data-extraction.md`](../schedules/main/data-extraction.md).
> §3 ("Date Filtering") and §6 ("Read-Time Staleness Guard") below describe the **original,
> now-obsolete** design and are kept only as a record of what changed and why; the engine no
> longer does either. The rest of this document (matching by week/day/subject, the Selective
> Left Join, discarding group-only lessons) is unchanged and current.

## 1. The Problem: Discrepancies Across Schedules

KPI students face a disjointed scheduling reality:

| Metric | Personal Schedule (`my.kpi.ua`) | Group Schedule (`api.campus.kpi.ua`) |
| :--- | :--- | :--- |
| **Authentication** | Required (Yii2 session cookies) | Public (no auth) |
| **Format** | FullCalendar JSON feed (two-step: HTML shell → JSON events) | Structured REST JSON |
| **Course Scope** | **Personalized**: Only selected electives & subgroups, already exact-dated | **All Group Courses**: Includes unselected electives & foreign subgroups, week-1/week-2 pattern |
| **Classrooms** | Plain-text fallback (`locationPDF`) | Rich object: `title` ("5-306") and `uri` ("https://kpi.ua/k-5") |
| **Lecturers** | Plain-text (`descriptionRAW`) | Detailed object: `id` and `name` |
| **Occurrence Dates** | Exact — each event's own `start` date | Pattern-based: specific ISO dates list `dates: ["2026-09-07", ...]`, or every cycle of a week if empty |

---

## 2. Merging Algorithm

The engine performs a **Selective Left Join** where the Personal Schedule serves as the base source of truth for *enrollment and dates*, and the Group Schedule serves as the metadata provider for *enrichment* (lecturer, location, and tag correction) only.

```mermaid
flowchart TD
    A[Dated Personal Lesson] --> B{Find in Group Schedule}
    C[Group Schedule Lessons Matrix] --> B

    B -->|Match by Week, Day, Slot & Subject| D[Enriched Lesson: + lecturer, + location, tag corrected]
    B -->|No Match Found| E[Fallback: Raw Personal Lesson, Enriched = false]

    D --> F[Stored As-Is — Date Is Already Authoritative]
    E --> F
```

A group Pair with no matching personal lesson never enters this flow at all — see §5.

---

## 3. Detailed Matching Steps

### Step 1: Time & Matrix Alignment
Each personal lesson's own `Date` is converted into the same 2D coordinate space the group
schedule uses, so the two can be matched:
- **Week**: derived via `engine.WeekAt(referenceDate, referenceWeek, lesson.Date)`, anchored
  on the Campus API's `/time/current` at refresh time.
- **Day**: derived via `engine.ISODay(lesson.Date)` — `1` (Monday) .. `7` (Sunday).
- **Slot Index**: resolved from the lesson's own `StartTime` against Campus's official
  lesson-slot times (`engine.slotByTime`) — Pair 1 (`08:30`) .. Pair 7 (`20:00`).

### Step 2: Subject String Normalization
Subject names often differ slightly between systems (e.g. trailing spaces, Roman vs Arabic numerals, abbreviations):
1. **Clean**: Convert to lower case, replace `’` with `'`, trim punctuation and double spaces (`engine.NormalizeSubject`).
2. **Normalize Types**: my.kpi.ua's own type codes (`lec`, `prc`, `lab`) are mapped to the Campus tag vocabulary (`lec`, `prac`, `lab`) via `mykpi.normalizeMyKPITag` — note `"prc"→"prac"`, the two sources use different short codes for practicals.
3. **Fuzzy Scoring**: Use Jaro-Winkler similarity (threshold $\ge 0.85$) if an exact normalized-name match is not found (`engine.jaroWinkler`).

### ~~Step 3: Date Filtering~~ (obsolete)

The original design filtered lessons by checking the target date against the Campus API's
`dates[]` array. This is no longer needed: since the personal source already carries an
exact `Date` per lesson, that date *is* the answer — there is nothing left to resolve. A
day/week read is a direct query on the stored `date` column
(`apps/server/internal/storage/lessons.go`, `GetLessonsByDateRange`).

---

## 4. Fallback Handling

If the Campus API is unreachable or the student belongs to a group whose schedule is not populated:
- The system returns the raw personal schedule from `my.kpi.ua`, unenriched.
- The response is flagged `enrichment_status: "degraded"`.
- No fatal crash occurs.

## 5. Discarding Group-Only Lessons

A group Pair with no matching personal lesson (same week, day, slot, and normalized subject)
is **discarded** — it is never stored and never shown. This is what keeps electives the
student didn't choose, and the alternate subgroup's lab session, out of their feed. See
`apps/server/internal/engine/merge.go` (`Merge`), covered by
`TestMergeDiscardsGroupOnlyLessons`.

## 6. ~~Read-Time Staleness Guard~~ (obsolete)

The original design re-fetched the group schedule on every read to re-derive `dates[]` live,
in case the Campus API's pattern-based dates had shifted between refreshes. This mechanism
existed only to answer "does this pattern-based lesson occur on this specific date?" — a
question the personal source's own `Date` field now answers directly, at refresh time, with
no live re-derivation needed. See `docs/architecture/data-storage.md` §5 for the full
rationale and the refresh-time enrichment flow that replaced it.
