package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/engine"
	"kpi-schedule-bot/server/internal/model"
)

// buildDay assembles the response for one calendar date: resolves the target
// week via the Campus API's current-time endpoint, filters stored lessons to
// that day, and re-verifies each enriched lesson's occurrence dates against
// a live group-schedule fetch (docs/architecture/merging-engine.md §2,
// staleness guard) so a lesson that no longer occurs on this date is dropped
// even if the stored snapshot hasn't been refreshed.
func (s *Service) buildDay(ctx context.Context, user model.User, targetDate time.Time, forceRefresh bool) (dayView, error) {
	stale, sessionStatus, enrichment, err := s.ensureFresh(ctx, user, forceRefresh)
	if err != nil {
		return dayView{}, err
	}

	week, err := s.resolveWeek(ctx, targetDate)
	if err != nil {
		return dayView{}, err
	}
	isoDay := engine.ISODay(targetDate)

	lessons, err := s.db.GetLessons(ctx, user.ID, week)
	if err != nil {
		return dayView{}, fmt.Errorf("loading lessons: %w", err)
	}

	groupSchedule, haveGroup := s.tryFetchGroupSchedule(ctx, user)

	views := make([]lessonView, 0)
	for _, l := range lessons {
		if l.Day != isoDay {
			continue
		}

		dates := l.Dates
		if l.Enriched && haveGroup {
			if fresh, ok := engine.RelookupDates(groupSchedule, l.Week, l.Day, l.SubjectNorm, l.Tag); ok {
				dates = fresh
			}
		}

		if !engine.OccursOn(dates, targetDate) {
			continue
		}
		l.Dates = dates
		views = append(views, toLessonView(l))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Slot < views[j].Slot })

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

	for _, w := range weeksToBuild {
		lessons, err := s.db.GetLessons(ctx, user.ID, w)
		if err != nil {
			return weekView{}, fmt.Errorf("loading lessons for week %d: %w", w, err)
		}

		byDay := make(map[int][]lessonView)
		for _, l := range lessons {
			byDay[l.Day] = append(byDay[l.Day], toLessonView(l))
		}

		var days []weekDayView
		for day := 1; day <= 6; day++ {
			ls := byDay[day]
			if len(ls) == 0 {
				continue
			}
			sort.Slice(ls, func(i, j int) bool { return ls[i].Slot < ls[j].Slot })
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

func (s *Service) tryFetchGroupSchedule(ctx context.Context, user model.User) (campus.GroupScheduleResponse, bool) {
	if user.GroupID == nil {
		return campus.GroupScheduleResponse{}, false
	}
	sched, err := s.campus.GroupSchedule(ctx, *user.GroupID)
	if err != nil {
		return campus.GroupScheduleResponse{}, false
	}
	return sched, true
}
