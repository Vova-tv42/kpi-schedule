package mykpi

import (
	"bytes"
	"fmt"

	"github.com/PuerkitoBio/goquery"
)

// ParsedLesson is one lesson occurrence pattern read from the personal calendar,
// before any enrichment from the Campus API.
type ParsedLesson struct {
	Week    int    // 1 or 2
	Day     int    // 1 (Monday) to 6 (Saturday)
	Slot    int    // 1 to 7
	Subject string // cleaned subject name
	Tag     string // "lec", "prac", "lab", or "" if undetected
	Type    string // original type label as shown on the page
	RawText string // full original block text, kept for debugging mismatches
}

// ErrNotImplemented is returned until the parser is finished against a real
// HTML fixture captured via POST /api/v1/debug/mykpi/dump. See
// docs/schedules/main/data-extraction.md for the handoff.
var ErrNotImplemented = fmt.Errorf("mykpi: calendar parser not yet implemented — capture a fixture via /api/v1/debug/mykpi/dump first")

// ParseCalendarHTML extracts the student's personal lessons from the raw HTML
// of https://my.kpi.ua/room/student/calendar.
func ParseCalendarHTML(htmlContent []byte) ([]ParsedLesson, error) {
	if _, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlContent)); err != nil {
		return nil, fmt.Errorf("loading HTML: %w", err)
	}

	// TODO(fixture): selectors below are placeholders pending a verified
	// HTML dump; see ErrNotImplemented.
	return nil, ErrNotImplemented
}
