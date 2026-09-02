package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	"kpi-schedule-bot/server/internal/engine"
	"kpi-schedule-bot/server/internal/model"
)

// BuildDay assembles the response for one calendar date. Since a schedule
// arrives as a client push (see docs/architecture/data-storage.md §1) rather
// than a server-side fetch, this only ever reads what is already stored —
// there is no inline refresh path. Exported for reuse by both the HTTP
// schedule handlers and the Telegram bot (internal/bot), which calls this
// in-process rather than looping back through HTTP.
func (s *Service) BuildDay(ctx context.Context, user model.User, targetDate time.Time) (dayView, error) {
	hasData, stale, enrichment, err := s.ScheduleFreshness(ctx, user)
	if err != nil {
		return dayView{}, err
	}
	if !hasData {
		return dayView{}, ErrNoScheduleData
	}

	dayStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.UTC)
	lessons, err := s.db.GetLessonsByDateRange(ctx, user.ID, dayStart, dayStart)
	if err != nil {
		return dayView{}, fmt.Errorf("loading lessons: %w", err)
	}

	views := make([]lessonView, 0, len(lessons))
	for _, l := range lessons {
		views = append(views, toLessonView(l))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Time < views[j].Time })

	isoDay := engine.ISODay(targetDate)
	week, err := s.ResolveWeekParity(ctx, targetDate)
	if err != nil {
		return dayView{}, err
	}

	return dayView{
		Date:             targetDate.Format("2006-01-02"),
		Week:             week,
		DayName:          dayNamesUA[isoDay],
		DayShort:         dayShortUA[isoDay],
		IsDayOff:         len(views) == 0,
		EnrichmentStatus: string(enrichment),
		Stale:            stale,
		Lessons:          views,
	}, nil
}

// BuildWeek assembles the response for a full academic week (or both).
// Exported for the same reason as BuildDay above.
func (s *Service) BuildWeek(ctx context.Context, user model.User, weekFilter int) (weekView, error) {
	hasData, stale, enrichment, err := s.ScheduleFreshness(ctx, user)
	if err != nil {
		return weekView{}, err
	}
	if !hasData {
		return weekView{}, ErrNoScheduleData
	}

	currentTime, err := s.campus.CurrentTime(ctx)
	if err != nil {
		return weekView{}, fmt.Errorf("resolving current academic week: %w", err)
	}

	weeksToBuild := []int{1, 2}
	if weekFilter == 1 || weekFilter == 2 {
		weeksToBuild = []int{weekFilter}
	}

	weekNames := map[int]string{1: "Перший тиждень (Чисельник)", 2: "Другий тиждень (Знаменник)"}

	out := weekView{
		TelegramID:       user.TelegramID,
		CurrentWeek:      currentTime.CurrentWeek,
		EnrichmentStatus: string(enrichment),
		Stale:            stale,
	}

	// Pull a window wide enough to cover both academic-week parities around
	// today, then bucket by the stored week/day fields.
	now := time.Now().UTC()
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -21)
	windowEnd := windowStart.AddDate(0, 0, 42)
	lessons, err := s.db.GetLessonsByDateRange(ctx, user.ID, windowStart, windowEnd)
	if err != nil {
		return weekView{}, fmt.Errorf("loading lessons: %w", err)
	}

	for _, w := range weeksToBuild {
		byDay := make(map[int][]lessonView)
		// The scan window above spans several real calendar weeks of the
		// same parity, so a recurring lesson has one stored row per
		// occurrence — dedupe down to a single representative per
		// (day, time, subject) slot. Irregular lessons (IsRecurring=false:
		// they only occur on a handful of specific dates, not every week —
		// see docs/architecture/merging-engine.md §6) are excluded entirely;
		// they still surface correctly via /today, /tomorrow, and /date,
		// which read the exact stored date directly.
		seen := make(map[string]bool)
		for _, l := range lessons {
			if l.Week != w || !l.IsRecurring {
				continue
			}
			key := fmt.Sprintf("%d|%s|%s", l.Day, l.StartTime, l.SubjectNorm)
			if seen[key] {
				continue
			}
			seen[key] = true
			byDay[l.Day] = append(byDay[l.Day], toLessonView(l))
		}

		var days []weekDayView
		for day := 1; day <= 7; day++ {
			ls := byDay[day]
			if len(ls) == 0 {
				continue
			}
			sort.Slice(ls, func(i, j int) bool { return ls[i].Time < ls[j].Time })
			days = append(days, weekDayView{
				Day:     dayShortUA[day],
				DayName: dayNamesUA[day],
				Lessons: ls,
			})
		}

		out.Weeks = append(out.Weeks, weekBlockView{
			WeekNumber: w,
			WeekName:   weekNames[w],
			Days:       days,
		})
	}

	return out, nil
}

// ResolveWeekParity derives the KPI week parity (1 or 2) for an arbitrary
// date from the Campus API's current-time anchor (see engine.WeekAt).
// Exported so the Telegram bot's /week navigation can turn a calendar-week
// offset into the parity BuildWeek expects.
func (s *Service) ResolveWeekParity(ctx context.Context, target time.Time) (int, error) {
	currentTime, err := s.campus.CurrentTime(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolving current academic week: %w", err)
	}
	return engine.WeekAt(time.Now(), currentTime.CurrentWeek, target), nil
}
