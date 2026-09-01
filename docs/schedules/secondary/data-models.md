# Campus API Data Models & Schemas

## 1. Domain Structs (Golang)

Below are the Go struct definitions matching the responses from `api.campus.kpi.ua`:

```go
package campus

// CurrentAcademicTime represents the response from /time/current
type CurrentAcademicTime struct {
    CurrentWeek   int `json:"currentWeek"`   // 1 or 2
    CurrentDay    int `json:"currentDay"`    // 1 (Monday) to 7 (Sunday)
    CurrentLesson int `json:"currentLesson"` // Current active slot (1..7)
}

// Group represents an item from the group catalog
type Group struct {
    ID      int    `json:"id"`
    Name    string `json:"name"`    // e.g. "ІП-21"
    Faculty string `json:"faculty"` // e.g. "ФІОТ"
}

// Lecturer represents instructor metadata
type Lecturer struct {
    ID   string `json:"id"`   // Hash identifier
    Name string `json:"name"` // Full Ukrainian name (e.g. "Стативка Юрій Іванович")
}

// Location represents physical classroom and building information
type Location struct {
    Title string `json:"title"` // e.g. "5-306" (Building 5, Room 306)
    URI   string `json:"uri"`   // Map / floor plan URL (e.g. "https://kpi.ua/k-5")
}

// Pair represents an individual class / lesson in the timetable
type Pair struct {
    Name     string    `json:"name"`     // Subject name (e.g. "Основи розробки трансляторів")
    Type     string    `json:"type"`     // "Лек", "Прак", "Лаб"
    Tag      string    `json:"tag"`      // "lec", "prac", "lab"
    Time     string    `json:"time"`     // "HH:MM:SS" (e.g. "18:05:00")
    Lecturer *Lecturer `json:"lecturer"` // Can be null if unassigned
    Location *Location `json:"location"` // Can be null for remote classes
    Dates    []string  `json:"dates"`    // Array of ISO "YYYY-MM-DD" dates (empty if every week)
}

// DaySchedule represents lessons scheduled for a specific day
type DaySchedule struct {
    Day   string `json:"day"`   // "Пн", "Вв", "Ср", "Чт", "Пт", "Сб"
    Pairs []Pair `json:"pairs"` // Ordered list of lessons
}

// GroupScheduleResponse represents the complete payload from /schedule/lessons
type GroupScheduleResponse struct {
    ScheduleFirstWeek  []DaySchedule `json:"scheduleFirstWeek"`
    ScheduleSecondWeek []DaySchedule `json:"scheduleSecondWeek"`
}
```

---

## 2. Key Data Fields Explained

### 2.1 Lesson Tags (`tag`)
- `"lec"`: Lecture (Лекція)
- `"prac"`: Practical / seminar session (Практика)
- `"lab"`: Laboratory work (Лабораторна)

### 2.2 Location Mapping (`location`)
- `location.title` is formatted as `{Building}-{Room}` (e.g., `18-402`, `5-306`, `7-05`).
- If a class is conducted entirely online (Zoom / Google Meet), `location` is `null`.

### 2.3 Occurrence Dates (`dates`)
- An empty array `[]` indicates the class occurs **every cycle** of that week (e.g. every Week 1 Tuesday).
- A populated array `["2026-09-07", "2026-09-21", ...]` indicates the class occurs **only** on the dates listed.
