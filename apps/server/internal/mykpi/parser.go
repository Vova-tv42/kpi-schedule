package mykpi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ParsedLesson is one concrete, dated lesson occurrence read from
// /calendar/studevents?id=<studentId>. Unlike the group schedule, these are
// already exact-dated and already filtered to only this student's actual
// enrollment — no elective/subgroup resolution is needed on our side.
type ParsedLesson struct {
	Date        time.Time // calendar date (UTC midnight)
	StartTime   string    // "HH:MM:SS"
	EndTime     string    // "HH:MM:SS"
	Subject     string
	Tag         string // normalized: "lec", "prac", "lab", or "" if unrecognized
	TeacherRaw  string
	LocationRaw string
}

// apiEvent mirrors one element of the FullCalendar events-JSON array
// returned by the studevents endpoint, keeping only the fields the parser
// needs. See docs/schedules/main/data-extraction.md for the full observed
// shape (captured via testdata/events-manual-*.json).
type apiEvent struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Start          string `json:"start"` // "2026-09-19T08:30:00"
	End            string `json:"end"`   // "2026-09-19T10:05:00"
	DescriptionRAW string `json:"descriptionRAW"`
	ExtendedProps  struct {
		Type        string `json:"type"`        // "lec", "prc" (not "prac"), possibly "lab"
		LocationPDF string `json:"locationPDF"` // plain-text location fallback, e.g. "lec., Онлайн Zoom"
	} `json:"extendedProps"`
}

const eventTimeLayout = "2006-01-02T15:04:05"

// ParseEventsJSON extracts the student's dated lessons from the raw JSON
// payload of https://my.kpi.ua/calendar/studevents?id=<studentId>.
func ParseEventsJSON(eventsJSON []byte) ([]ParsedLesson, error) {
	var events []apiEvent
	if err := json.Unmarshal(eventsJSON, &events); err != nil {
		return nil, fmt.Errorf("unmarshaling events JSON: %w", err)
	}

	lessons := make([]ParsedLesson, 0, len(events))
	for _, e := range events {
		startAt, err := time.Parse(eventTimeLayout, e.Start)
		if err != nil {
			return nil, fmt.Errorf("parsing start time %q for event %d: %w", e.Start, e.ID, err)
		}

		var endTime string
		if e.End != "" {
			endAt, err := time.Parse(eventTimeLayout, e.End)
			if err != nil {
				return nil, fmt.Errorf("parsing end time %q for event %d: %w", e.End, e.ID, err)
			}
			endTime = endAt.Format("15:04:05")
		}

		lessons = append(lessons, ParsedLesson{
			Date:        time.Date(startAt.Year(), startAt.Month(), startAt.Day(), 0, 0, 0, 0, time.UTC),
			StartTime:   startAt.Format("15:04:05"),
			EndTime:     endTime,
			Subject:     strings.TrimSpace(e.Title),
			Tag:         normalizeMyKPITag(e.ExtendedProps.Type),
			TeacherRaw:  parseTeacherRaw(e.DescriptionRAW),
			LocationRaw: strings.TrimSpace(e.ExtendedProps.LocationPDF),
		})
	}
	return lessons, nil
}

// normalizeMyKPITag maps my.kpi.ua's own type codes to the Campus API's tag
// vocabulary ("lec", "prac", "lab") so the two sources can be matched during
// enrichment — notably, my.kpi.ua uses "prc" where Campus uses "prac".
func normalizeMyKPITag(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "lec":
		return "lec"
	case "prc", "prac":
		return "prac"
	case "lab":
		return "lab"
	default:
		return ""
	}
}

// parseTeacherRaw strips the "Викладачі: " label my.kpi.ua prefixes onto
// descriptionRAW, if present, keeping the raw text otherwise.
func parseTeacherRaw(descriptionRAW string) string {
	s := strings.TrimSpace(descriptionRAW)
	const prefix = "Викладачі:"
	if strings.HasPrefix(s, prefix) {
		return strings.TrimSpace(s[len(prefix):])
	}
	return s
}
