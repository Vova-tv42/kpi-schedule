package engine

import "time"

// mondayOf returns the Monday (00:00 UTC) of the ISO week containing t.
func mondayOf(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	// time.Weekday: Sunday=0 .. Saturday=6. Convert to ISO (Monday=0..Sunday=6).
	isoWeekday := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -isoWeekday)
}

// WeekAt derives the KPI academic week parity (1 or 2) for targetDate, given
// that referenceDate falls in referenceWeek. KPI's week-1/week-2 cycle
// flips every 7 days (see docs/architecture/merging-engine.md §3, Step 1),
// so this counts whole weeks between the two dates' Mondays and flips parity
// on every odd offset — deliberately not using time.Time.ISOWeek() directly,
// since its week numbers reset at year boundaries in a way that would
// desynchronize the parity across a semester spanning two years.
func WeekAt(referenceDate time.Time, referenceWeek int, targetDate time.Time) int {
	refMonday := mondayOf(referenceDate)
	targetMonday := mondayOf(targetDate)

	days := int(targetMonday.Sub(refMonday).Hours() / 24)
	weeksOffset := days / 7

	// weeksOffset is exact because both inputs are Mondays 7 days apart.
	parity := ((referenceWeek - 1) + weeksOffset) % 2
	if parity < 0 {
		parity += 2
	}
	return parity + 1
}

// ISODay returns the ISO weekday of t as used by the schema: 1 (Monday) .. 7 (Sunday).
func ISODay(t time.Time) int {
	return (int(t.Weekday())+6)%7 + 1
}

// OccursOn reports whether a lesson with the given occurrence dates occurs on
// target, per docs/architecture/merging-engine.md §3, Step 3: an empty dates
// list means "every cycle of this week"; a non-empty list is exhaustive.
func OccursOn(dates []string, target time.Time) bool {
	if len(dates) == 0 {
		return true
	}
	targetStr := target.Format("2006-01-02")
	for _, d := range dates {
		if d == targetStr {
			return true
		}
	}
	return false
}
