# Scraping & Data Extraction Strategy (my.kpi.ua)

## 1. Scraping Target

- **Endpoint**: `https://my.kpi.ua/room/student/calendar`
- **Method**: `GET`
- **Headers Required**:
  - `Cookie: PHPSESSID=...; _identity=...`
  - `User-Agent: Mozilla/5.0 ...`
- **Output Format**: HTML document containing the student's personal calendar matrix.

---

## 2. DOM Structure & CSS Selectors

Based on style analysis from `/css/site.css` and `/css/print_schedule.css`, `my.kpi.ua` organizes the calendar into weekly tables or grid containers:

```text
Table / Grid Structure:
├── Week Container (Odd Week / Even Week) [.odd-week, .even-week]
│   ├── Day Columns (Monday -> Saturday) [.dow_h, .c_cell]
│   │   ├── Lesson Time Slot (Pair 1..7)
│   │   │   ├── Subject Title
│   │   │   ├── Lesson Type Indicator (Лек / Прак / Лаб)
│   │   │   └── Teacher / Classroom info (if provided)
```

### Key CSS Classes & Attributes
- `.odd-week` / `.even-week`: Selectors indicating Week 1 (numerator / чисельник) vs Week 2 (denominator / знаменник).
- `.c_body`, `.c_cell`, `.ui-table-modern`: Table and cell containers for schedule matrix blocks.
- `.dow_h`, `.dowh-fixed`: Day of week header row (Пн, Вв, Ср, Чт, Пт, Сб).

---

## 3. Goquery Parsing Workflow in Golang

```go
package mykpi

import (
    "bytes"
    "fmt"
    "strings"
    "github.com/PuerkitoBio/goquery"
)

type ParsedLesson struct {
    WeekNumber int    // 1 or 2
    DayOfWeek  int    // 1 (Monday) to 6 (Saturday)
    SlotNumber int    // 1 to 7
    Subject    string // Cleaned subject name
    TypeTag    string // "lec", "prac", "lab"
    RawText    string // Full original block text
}

func ParseCalendarHTML(htmlContent []byte) ([]ParsedLesson, error) {
    doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent))
    if err != nil {
        return nil, fmt.Errorf("loading HTML: %w", err)
    }

    var lessons []ParsedLesson

    // 1. Locate week sections (Week 1 / Week 2)
    // 2. Iterate through day rows / columns
    // 3. Extract time slots and subject titles
    // 4. Normalize subject strings and classify tags

    return lessons, nil
}
```

---

## 4. Extraction & Normalization Rules

1. **Tag Detection**:
   - Matches words like `Лекція`, `Лек.`, `(Лек)` $\rightarrow$ `"lec"`
   - Matches words like `Практика`, `Прак.`, `(Прак)` $\rightarrow$ `"prac"`
   - Matches words like `Лабораторна`, `Лаб.`, `(Лаб)` $\rightarrow$ `"lab"`

2. **Clean Whitespace & Punctuation**:
   - Strip leading/trailing whitespaces, newlines, and non-breaking spaces (`&nbsp;`).
   - Remove redundant brackets, room prefixes, or subgroup labels for the base subject key.

3. **Validation Threshold**:
   - If the parsed lesson count is `0`, verify whether it is semester break / exam period or if the DOM structure changed.
   - If DOM structure has unexpectedly changed, trigger a resilience alert (see `docs/architecture/error-handling-resilience.md`).
