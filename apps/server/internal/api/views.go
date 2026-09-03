package api

import (
	"kpi-schedule-bot/server/internal/model"
)

var dayNamesUA = map[int]string{
	1: "Понеділок", 2: "Вівторок", 3: "Середа", 4: "Четвер", 5: "П'ятниця", 6: "Субота", 7: "Неділя",
}

var dayShortUA = map[int]string{
	1: "Пн", 2: "Вв", 3: "Ср", 4: "Чт", 5: "Пт", 6: "Сб", 7: "Нд",
}

type lecturerView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type locationView struct {
	Title string `json:"title"`
	URI   string `json:"uri"`
}

type lessonView struct {
	Date        string        `json:"date"`
	Slot        int           `json:"slot"`
	Time        string        `json:"time"`
	EndTime     string        `json:"end_time"`
	Name        string        `json:"name"`
	Tag         string        `json:"tag"`
	TeacherRaw  string        `json:"teacher_raw,omitempty"`
	LocationRaw string        `json:"location_raw,omitempty"`
	Lecturer    *lecturerView `json:"lecturer,omitempty"`
	Location    *locationView `json:"location,omitempty"`
	Enriched    bool          `json:"enriched"`
	URL         string        `json:"url,omitempty"`
}

func toLessonView(l model.Lesson, url string) lessonView {
	v := lessonView{
		Date:        l.Date.Format("2006-01-02"),
		Slot:        l.Slot,
		Time:        l.StartTime,
		EndTime:     l.EndTime,
		Name:        l.Subject,
		Tag:         l.Tag,
		TeacherRaw:  l.TeacherRaw,
		LocationRaw: l.LocationRaw,
		Enriched:    l.Enriched,
		URL:         url,
	}
	if l.Lecturer != nil {
		v.Lecturer = &lecturerView{ID: l.Lecturer.ID, Name: l.Lecturer.Name}
	}
	if l.Location != nil {
		v.Location = &locationView{Title: l.Location.Title, URI: l.Location.URI}
	}
	return v
}

type dayView struct {
	Date             string       `json:"date"`
	Week             int          `json:"week"`
	DayName          string       `json:"day_name"`
	DayShort         string       `json:"day_short"`
	IsDayOff         bool         `json:"is_day_off"`
	EnrichmentStatus string       `json:"enrichment_status"`
	Stale            bool         `json:"stale"`
	Lessons          []lessonView `json:"lessons"`
}

type weekDayView struct {
	Day     string       `json:"day"`
	DayName string       `json:"day_name"`
	Lessons []lessonView `json:"lessons"`
}

type weekBlockView struct {
	WeekNumber int           `json:"week_number"`
	WeekName   string        `json:"week_name"`
	Days       []weekDayView `json:"days"`
}

type weekView struct {
	TelegramID       int64           `json:"telegram_id"`
	CurrentWeek      int             `json:"current_week"`
	EnrichmentStatus string          `json:"enrichment_status"`
	Stale            bool            `json:"stale"`
	Weeks            []weekBlockView `json:"weeks"`
}
