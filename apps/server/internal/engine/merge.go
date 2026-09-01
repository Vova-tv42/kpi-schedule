package engine

import (
	"time"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/mykpi"
)

// jaroWinkler returns a similarity score in [0,1]. Used as a fallback when an
// exact normalized-subject match is not found (docs/architecture/merging-engine.md §3, Step 2).
func jaroWinkler(a, b string) float64 {
	if a == b {
		return 1
	}
	aLen, bLen := len(a), len(b)
	if aLen == 0 || bLen == 0 {
		return 0
	}

	matchDistance := max(aLen, bLen)/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	aMatches := make([]bool, aLen)
	bMatches := make([]bool, bLen)

	matches := 0
	for i := 0; i < aLen; i++ {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, bLen)
		for j := start; j < end; j++ {
			if bMatches[j] || a[i] != b[j] {
				continue
			}
			aMatches[i] = true
			bMatches[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}

	transpositions := 0
	k := 0
	for i := 0; i < aLen; i++ {
		if !aMatches[i] {
			continue
		}
		for !bMatches[k] {
			k++
		}
		if a[i] != b[k] {
			transpositions++
		}
		k++
	}
	transpositions /= 2

	m := float64(matches)
	jaro := (m/float64(aLen) + m/float64(bLen) + (m-float64(transpositions))/m) / 3

	// Winkler prefix bonus: up to 4 leading matching chars, scale 0.1.
	prefix := 0
	for i := 0; i < min(4, min(aLen, bLen)); i++ {
		if a[i] != b[i] {
			break
		}
		prefix++
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}

// dayIndex maps a Campus API day label to the schema's 1..7 (Monday..Sunday).
var dayIndex = map[string]int{
	"Пн": 1, "Вв": 2, "Ср": 3, "Чт": 4, "Пт": 5, "Сб": 6, "Нд": 7,
}

// groupSlot pairs a group Pair with the day/week it belongs to, flattened for lookup.
type groupSlot struct {
	week int
	day  int
	pair campus.Pair
}

func flattenGroupSchedule(sched campus.GroupScheduleResponse) []groupSlot {
	var slots []groupSlot
	for _, ds := range sched.ScheduleFirstWeek {
		day, ok := dayIndex[ds.Day]
		if !ok {
			continue
		}
		for _, p := range ds.Pairs {
			slots = append(slots, groupSlot{week: 1, day: day, pair: p})
		}
	}
	for _, ds := range sched.ScheduleSecondWeek {
		day, ok := dayIndex[ds.Day]
		if !ok {
			continue
		}
		for _, p := range ds.Pairs {
			slots = append(slots, groupSlot{week: 2, day: day, pair: p})
		}
	}
	return slots
}

// MergedLesson is a personal, dated lesson after enrichment, ready to persist.
type MergedLesson struct {
	Date        time.Time
	Week        int
	Day         int
	Slot        int
	StartTime   string
	EndTime     string
	Subject     string
	SubjectNorm string
	Tag         string
	TeacherRaw  string
	LocationRaw string
	Lecturer    *campus.Lecturer
	Location    *campus.Location
	Enriched    bool
}

// slotByTime resolves a HH:MM:SS time to a 1..7 slot number using the
// Campus API's official slot times (see campus.Client.LessonSlots). Returns
// ok=false (slot 0) if the time doesn't match any known slot.
func slotByTime(slots map[string]string, t string) (int, bool) {
	for slot, slotTime := range slots {
		if slotTime == t {
			n := 0
			for _, c := range slot {
				if c < '0' || c > '9' {
					return 0, false
				}
				n = n*10 + int(c-'0')
			}
			return n, true
		}
	}
	return 0, false
}

// findMatch locates the best-scoring group pair for a personal lesson's
// week/day/subject, per docs/architecture/merging-engine.md §3, Step 2.
func findMatch(groupSlots []groupSlot, week, day int, subjectNorm, tag string) *campus.Pair {
	var best *groupSlot
	bestScore := 0.0
	for i := range groupSlots {
		gs := &groupSlots[i]
		if gs.week != week || gs.day != day {
			continue
		}
		gNorm := NormalizeSubject(gs.pair.Name)

		var score float64
		switch {
		case gNorm == subjectNorm:
			score = 1
		default:
			score = jaroWinkler(subjectNorm, gNorm)
		}
		if tag != "" && gs.pair.Tag != "" && tag == gs.pair.Tag {
			score += 0.001 // tie-break toward matching tag without overriding a stronger name match
		}
		if score > bestScore && (gNorm == subjectNorm || score >= 0.85) {
			bestScore = score
			best = gs
		}
	}
	if best == nil {
		return nil
	}
	return &best.pair
}

// Merge implements the Selective Left Join from docs/architecture/merging-engine.md §2:
// the personal, dated schedule from my.kpi.ua is the base; the Campus group
// schedule only enriches matching lessons with lecturer/location metadata.
// A group lesson with no personal counterpart is discarded — never stored,
// never shown. week/day are derived per-lesson from its Date via WeekAt/ISODay,
// anchored on referenceDate/referenceWeek (the Campus API's current-time
// anchor at refresh time).
func Merge(personal []mykpi.ParsedLesson, group campus.GroupScheduleResponse, slotTimes map[string]string, referenceDate time.Time, referenceWeek int) []MergedLesson {
	groupSlots := flattenGroupSchedule(group)

	merged := make([]MergedLesson, 0, len(personal))
	for _, p := range personal {
		week := WeekAt(referenceDate, referenceWeek, p.Date)
		day := ISODay(p.Date)
		pNorm := NormalizeSubject(p.Subject)
		best := findMatch(groupSlots, week, day, pNorm, p.Tag)

		m := MergedLesson{
			Date:        p.Date,
			Week:        week,
			Day:         day,
			StartTime:   p.StartTime,
			EndTime:     p.EndTime,
			Subject:     p.Subject,
			SubjectNorm: pNorm,
			Tag:         p.Tag,
			TeacherRaw:  p.TeacherRaw,
			LocationRaw: p.LocationRaw,
		}

		if slot, ok := slotByTime(slotTimes, p.StartTime); ok {
			m.Slot = slot
		}

		if best != nil {
			if best.Tag != "" {
				m.Tag = best.Tag
			}
			m.Lecturer = best.Lecturer
			m.Location = best.Location
			m.Enriched = true
		}

		merged = append(merged, m)
	}
	return merged
}
