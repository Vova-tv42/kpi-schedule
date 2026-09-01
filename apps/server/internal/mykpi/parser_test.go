package mykpi

import (
	"os"
	"testing"
)

func TestParseEventsJSONGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/events-golden.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	got, err := ParseEventsJSON(data)
	if err != nil {
		t.Fatalf("ParseEventsJSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d lessons, want 2", len(got))
	}

	l0 := got[0]
	if l0.Date.Format("2006-01-02") != "2026-09-05" {
		t.Errorf("l0.Date = %s, want 2026-09-05", l0.Date.Format("2006-01-02"))
	}
	if l0.StartTime != "08:30:00" || l0.EndTime != "10:05:00" {
		t.Errorf("l0 times = %s..%s, want 08:30:00..10:05:00", l0.StartTime, l0.EndTime)
	}
	if l0.Subject != "Технології DevOps" {
		t.Errorf("l0.Subject = %q", l0.Subject)
	}
	if l0.Tag != "lec" {
		t.Errorf("l0.Tag = %q, want lec", l0.Tag)
	}
	if l0.TeacherRaw != "Колумбет В. П." {
		t.Errorf("l0.TeacherRaw = %q, want %q", l0.TeacherRaw, "Колумбет В. П.")
	}
	if l0.LocationRaw != "lec., Онлайн Zoom" {
		t.Errorf("l0.LocationRaw = %q", l0.LocationRaw)
	}

	l1 := got[1]
	// my.kpi.ua's own type code is "prc", must normalize to Campus's "prac".
	if l1.Tag != "prac" {
		t.Errorf("l1.Tag = %q, want prac (normalized from my.kpi.ua's \"prc\")", l1.Tag)
	}
	if l1.Date.Format("2006-01-02") != "2026-09-08" {
		t.Errorf("l1.Date = %s, want 2026-09-08", l1.Date.Format("2006-01-02"))
	}
}

func TestParseEventsJSONEmpty(t *testing.T) {
	got, err := ParseEventsJSON([]byte(`[]`))
	if err != nil {
		t.Fatalf("ParseEventsJSON([]): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d lessons, want 0", len(got))
	}
}
