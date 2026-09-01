package engine

import (
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestWeekAtSameWeek(t *testing.T) {
	// 2026-09-01 is a Tuesday; 2026-09-03 (Thursday) is the same ISO week.
	got := WeekAt(date("2026-09-01"), 1, date("2026-09-03"))
	if got != 1 {
		t.Fatalf("got week %d, want 1", got)
	}
}

func TestWeekAtNextWeekFlips(t *testing.T) {
	got := WeekAt(date("2026-09-01"), 1, date("2026-09-08"))
	if got != 2 {
		t.Fatalf("got week %d, want 2", got)
	}
}

func TestWeekAtTwoWeeksLaterSameParity(t *testing.T) {
	got := WeekAt(date("2026-09-01"), 1, date("2026-09-15"))
	if got != 1 {
		t.Fatalf("got week %d, want 1", got)
	}
}

func TestWeekAtPastDateFlips(t *testing.T) {
	got := WeekAt(date("2026-09-08"), 2, date("2026-09-01"))
	if got != 1 {
		t.Fatalf("got week %d, want 1", got)
	}
}

func TestWeekAtAcrossYearBoundary(t *testing.T) {
	// Regression guard: ISO week numbers reset in early January, which could
	// desynchronize a naive ISOWeek()-based parity calculation.
	ref := date("2026-12-28") // Monday
	got := WeekAt(ref, 1, date("2027-01-04"))
	if got != 2 {
		t.Fatalf("got week %d, want 2", got)
	}
}

func TestISODay(t *testing.T) {
	cases := map[string]int{
		"2026-09-01": 2, // Tuesday reference date in this session
		"2026-08-31": 1, // Monday
		"2026-09-06": 7, // Sunday
	}
	for ds, want := range cases {
		got := ISODay(date(ds))
		if got != want {
			t.Errorf("ISODay(%s) = %d, want %d", ds, got, want)
		}
	}
}
