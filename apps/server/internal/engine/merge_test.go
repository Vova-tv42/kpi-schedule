package engine

import (
	"testing"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/model"
)

var testSlots = map[string]string{
	"1": "08:30:00", "2": "10:25:00", "3": "12:20:00",
	"4": "14:15:00", "5": "16:10:00", "6": "18:05:00", "7": "20:00:00",
}

// referenceDate/referenceWeek anchor the tests' week-parity math: 2026-09-01
// is a Tuesday in week 1 (per weeks_test.go's convention).
var referenceDate = date("2026-09-01")

const referenceWeek = 1

func TestMergeEnrichesMatchingLesson(t *testing.T) {
	// 2026-09-01 is a Tuesday (day 2) in week 1.
	personal := []model.ParsedLesson{
		{Date: date("2026-09-01"), StartTime: "10:25:00", Subject: "Технології DevOps", Tag: "prac"},
	}
	group := campus.GroupScheduleResponse{
		ScheduleFirstWeek: []campus.DaySchedule{
			{Day: "Вв", Pairs: []campus.Pair{
				{
					Name: "технології devops", Tag: "prac", Type: "Прак", Time: "10:25:00",
					Lecturer: &campus.Lecturer{ID: "l1", Name: "Колумбет В. П."},
					Location: &campus.Location{Title: "5-508", URI: "https://kpi.ua/k-5"},
				},
				{
					// Same slot, different subject — must not be picked.
					Name: "Комп'ютерна графіка", Tag: "prac", Time: "10:25:00",
				},
			}},
		},
	}

	got := Merge(personal, group, testSlots, referenceDate, referenceWeek)
	if len(got) != 1 {
		t.Fatalf("got %d lessons, want 1", len(got))
	}
	l := got[0]
	if !l.Enriched {
		t.Fatalf("expected lesson to be enriched")
	}
	if l.Lecturer == nil || l.Lecturer.Name != "Колумбет В. П." {
		t.Fatalf("lecturer not attached correctly: %+v", l.Lecturer)
	}
	if l.Location == nil || l.Location.Title != "5-508" {
		t.Fatalf("location not attached correctly: %+v", l.Location)
	}
	if l.Slot != 2 {
		t.Fatalf("got slot %d, want 2", l.Slot)
	}
	if l.Week != 1 || l.Day != 2 {
		t.Fatalf("got week %d day %d, want week 1 day 2", l.Week, l.Day)
	}
}

func TestMergeKeepsUnmatchedPersonalLesson(t *testing.T) {
	// 2026-08-31 is a Monday (day 1) in week 1.
	personal := []model.ParsedLesson{
		{Date: date("2026-08-31"), StartTime: "08:30:00", Subject: "Невідомий курс", Tag: "lec"},
	}
	group := campus.GroupScheduleResponse{}

	got := Merge(personal, group, testSlots, referenceDate, referenceWeek)
	if len(got) != 1 {
		t.Fatalf("got %d lessons, want 1", len(got))
	}
	if got[0].Enriched {
		t.Fatalf("expected lesson to remain unenriched")
	}
	if got[0].Subject != "Невідомий курс" {
		t.Fatalf("personal lesson subject was altered: %q", got[0].Subject)
	}
}

func TestMergeDiscardsGroupOnlyLessons(t *testing.T) {
	// No personal lessons at all — the group schedule's "Комп'ютерна графіка"
	// (an elective this student didn't choose) must never appear in the output.
	group := campus.GroupScheduleResponse{
		ScheduleFirstWeek: []campus.DaySchedule{
			{Day: "Пн", Pairs: []campus.Pair{
				{Name: "Комп'ютерна графіка", Tag: "prac", Time: "08:30:00"},
			}},
		},
	}

	got := Merge(nil, group, testSlots, referenceDate, referenceWeek)
	if len(got) != 0 {
		t.Fatalf("got %d lessons, want 0 (group-only lessons must be discarded)", len(got))
	}
}

func TestMergeDoesNotEnrichAcrossDifferentDayOrWeek(t *testing.T) {
	// 2026-09-09 is a Wednesday (day 3) in week 2 (one week after the
	// reference Tuesday in week 1).
	personal := []model.ParsedLesson{
		{Date: date("2026-09-09"), StartTime: "08:30:00", Subject: "Математика", Tag: "lec"},
	}
	group := campus.GroupScheduleResponse{
		ScheduleFirstWeek: []campus.DaySchedule{ // week 1, not week 2
			{Day: "Ср", Pairs: []campus.Pair{
				{Name: "Математика", Tag: "lec", Time: "08:30:00"},
			}},
		},
	}

	got := Merge(personal, group, testSlots, referenceDate, referenceWeek)
	if len(got) != 1 || got[0].Enriched {
		t.Fatalf("lesson must not match across a different week: %+v", got)
	}
	if got[0].Week != 2 {
		t.Fatalf("got week %d, want 2", got[0].Week)
	}
}
