// Package campus is a client for the public api.campus.kpi.ua REST API.
// Structs are transcribed from docs/schedules/secondary/data-models.md.
package campus

// CurrentAcademicTime is the response from GET /time/current.
type CurrentAcademicTime struct {
	CurrentWeek   int `json:"currentWeek"`   // 1 or 2
	CurrentDay    int `json:"currentDay"`    // 1 (Monday) to 7 (Sunday)
	CurrentLesson int `json:"currentLesson"` // Current active slot (1..7)
}

// Group is one entry from the group catalog.
type Group struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`    // e.g. "ІП-21"
	Faculty string `json:"faculty"` // e.g. "ФІОТ"
}

// Lecturer is instructor metadata.
type Lecturer struct {
	ID   string `json:"id"`   // Hash identifier
	Name string `json:"name"` // Full Ukrainian name
}

// Location is physical classroom/building information.
type Location struct {
	Title string `json:"title"` // e.g. "5-306" (Building 5, Room 306)
	URI   string `json:"uri"`   // Map / floor plan URL
}

// Pair is an individual class/lesson in the group timetable.
type Pair struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"` // "Лек", "Прак", "Лаб"
	Tag      string    `json:"tag"`  // "lec", "prac", "lab"
	Time     string    `json:"time"` // "HH:MM:SS"
	Lecturer *Lecturer `json:"lecturer"`
	Location *Location `json:"location"`
	Dates    []string  `json:"dates"` // ISO "YYYY-MM-DD"; empty = every cycle of that week
}

// DaySchedule is the lessons scheduled for one day.
type DaySchedule struct {
	Day   string `json:"day"` // "Пн", "Вв", "Ср", "Чт", "Пт", "Сб"
	Pairs []Pair `json:"pairs"`
}

// GroupScheduleResponse is the full payload from GET /schedule/lessons.
type GroupScheduleResponse struct {
	ScheduleFirstWeek  []DaySchedule `json:"scheduleFirstWeek"`
	ScheduleSecondWeek []DaySchedule `json:"scheduleSecondWeek"`
}
