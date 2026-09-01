package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	"kpi-schedule-bot/server/internal/engine"
	"kpi-schedule-bot/server/internal/model"
)

// buildDay assembles the response for one calendar date. Since my.kpi.ua's
// personal feed already returns exact-dated lesson instances, the stored
// `date` column is authoritative and queried directly — no read-time
// re-verification against the group schedule is needed (that was only ever
// necessary to re-derive occurrence dates from the Campus API's week-pattern
// `dates[]`, which the personal schedule no longer needs).
func (s *Service) buildDay(ctx context.Context, user model.User, targetDate time.Time, forceRefresh bool) (dayView, error) {
	stale, sessionStatus, enrichment, err := s.ensureFresh(ctx, user, forceRefresh)
	if err != nil {
		return dayView{}, err
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
	week, err := s.resolveWeek(ctx, targetDate)
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
		SessionStatus:    string(sessionStatus),
		Lessons:          views,
	}, nil
}

func (s *Service) buildWeek(ctx context.Context, user model.User, weekFilter int, forceRefresh bool) (weekView, error) {
	stale, sessionStatus, enrichment, err := s.ensureFresh(ctx, user, forceRefresh)
	if err != nil {
		return weekView{}, err
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
		SessionStatus:    string(sessionStatus),
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
		for _, l := range lessons {
			if l.Week != w {
				continue
			}
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

// resolveWeek derives the KPI week parity for an arbitrary date from the
// Campus API's current-time anchor (see engine.WeekAt).
func (s *Service) resolveWeek(ctx context.Context, target time.Time) (int, error) {
	currentTime, err := s.campus.CurrentTime(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolving current academic week: %w", err)
	}
	return engine.WeekAt(time.Now(), currentTime.CurrentWeek, target), nil
}
