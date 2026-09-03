package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/campus"
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

	urls, err := s.db.GetLessonURLs(ctx, user.ID)
	if err != nil {
		return dayView{}, fmt.Errorf("loading lesson urls: %w", err)
	}

	views := make([]lessonView, 0, len(lessons))
	for _, l := range lessons {
		url := urls[l.SubjectNorm+"|"+l.Tag]
		views = append(views, toLessonView(l, url))
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

	urls, err := s.db.GetLessonURLs(ctx, user.ID)
	if err != nil {
		return weekView{}, fmt.Errorf("loading lesson urls: %w", err)
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
			url := urls[l.SubjectNorm+"|"+l.Tag]
			byDay[l.Day] = append(byDay[l.Day], toLessonView(l, url))
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

// BuildGroupDay fetches and formats a group's schedule for a single calendar date
// directly from the secondary Campus API, attaching any group custom URLs.
func (s *Service) BuildGroupDay(ctx context.Context, botGroupID *uuid.UUID, groupID int, targetDate time.Time) (dayView, error) {
	parity, err := s.ResolveWeekParity(ctx, targetDate)
	if err != nil {
		return dayView{}, fmt.Errorf("resolving week parity: %w", err)
	}

	sched, err := s.campus.GroupSchedule(ctx, groupID)
	if err != nil {
		return dayView{}, fmt.Errorf("fetching group schedule: %w", err)
	}

	var groupURLs map[string]string
	if botGroupID != nil {
		groupURLs, _ = s.db.GetGroupLessonURLs(ctx, *botGroupID)
	}

	var weekSchedule []campus.DaySchedule
	if parity == 1 {
		weekSchedule = sched.ScheduleFirstWeek
	} else {
		weekSchedule = sched.ScheduleSecondWeek
	}

	isoDay := engine.ISODay(targetDate)
	dayShort := dayShortUA[isoDay]

	var matchingDay *campus.DaySchedule
	for _, d := range weekSchedule {
		if d.Day == dayShort {
			dCopy := d
			matchingDay = &dCopy
			break
		}
	}

	targetDateStr := targetDate.Format("2006-01-02")
	var views []lessonView
	if matchingDay != nil {
		for _, p := range matchingDay.Pairs {
			if len(p.Dates) > 0 {
				found := false
				for _, d := range p.Dates {
					if d == targetDateStr {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			var lView *lecturerView
			if p.Lecturer != nil {
				lView = &lecturerView{ID: p.Lecturer.ID, Name: p.Lecturer.Name}
			}
			var locView *locationView
			if p.Location != nil {
				locView = &locationView{Title: p.Location.Title, URI: p.Location.URI}
			}
			teacherRaw := ""
			if p.Lecturer != nil {
				teacherRaw = p.Lecturer.Name
			}
			locRaw := ""
			if p.Location != nil {
				locRaw = p.Location.Title
			}

			norm := engine.NormalizeSubject(p.Name)
			tag := engine.NormalizeTag(p.Tag)
			url := ""
			if groupURLs != nil {
				url = groupURLs[norm+"|"+tag]
			}

			views = append(views, lessonView{
				Date:        targetDateStr,
				Time:        p.Time,
				Name:        p.Name,
				Tag:         p.Tag,
				TeacherRaw:  teacherRaw,
				LocationRaw: locRaw,
				Lecturer:    lView,
				Location:    locView,
				Enriched:    true,
				URL:         url,
			})
		}
	}

	sort.Slice(views, func(i, j int) bool { return views[i].Time < views[j].Time })

	return dayView{
		Date:             targetDateStr,
		Week:             parity,
		DayName:          dayNamesUA[isoDay],
		DayShort:         dayShort,
		IsDayOff:         len(views) == 0,
		EnrichmentStatus: "full",
		Stale:            false,
		Lessons:          views,
	}, nil
}

// BuildGroupWeek fetches and formats a group's schedule for a full academic week
// directly from the secondary Campus API, attaching any group custom URLs.
func (s *Service) BuildGroupWeek(ctx context.Context, botGroupID *uuid.UUID, groupID int, weekFilter int) (weekView, error) {
	currentTime, err := s.campus.CurrentTime(ctx)
	if err != nil {
		return weekView{}, fmt.Errorf("resolving current academic week: %w", err)
	}

	sched, err := s.campus.GroupSchedule(ctx, groupID)
	if err != nil {
		return weekView{}, fmt.Errorf("fetching group schedule: %w", err)
	}

	var groupURLs map[string]string
	if botGroupID != nil {
		groupURLs, _ = s.db.GetGroupLessonURLs(ctx, *botGroupID)
	}

	weeksToBuild := []int{1, 2}
	if weekFilter == 1 || weekFilter == 2 {
		weeksToBuild = []int{weekFilter}
	}

	weekNames := map[int]string{1: "Перший тиждень (Чисельник)", 2: "Другий тиждень (Знаменник)"}
	out := weekView{
		CurrentWeek:      currentTime.CurrentWeek,
		EnrichmentStatus: "full",
		Stale:            false,
	}

	for _, w := range weeksToBuild {
		var weekSched []campus.DaySchedule
		if w == 1 {
			weekSched = sched.ScheduleFirstWeek
		} else {
			weekSched = sched.ScheduleSecondWeek
		}

		dayMap := make(map[string][]campus.Pair)
		for _, d := range weekSched {
			dayMap[d.Day] = d.Pairs
		}

		var days []weekDayView
		for dayIdx := 1; dayIdx <= 7; dayIdx++ {
			short := dayShortUA[dayIdx]
			pairs := dayMap[short]
			if len(pairs) == 0 {
				continue
			}
			var views []lessonView
			for _, p := range pairs {
				var lView *lecturerView
				if p.Lecturer != nil {
					lView = &lecturerView{ID: p.Lecturer.ID, Name: p.Lecturer.Name}
				}
				var locView *locationView
				if p.Location != nil {
					locView = &locationView{Title: p.Location.Title, URI: p.Location.URI}
				}
				teacherRaw := ""
				if p.Lecturer != nil {
					teacherRaw = p.Lecturer.Name
				}
				locRaw := ""
				if p.Location != nil {
					locRaw = p.Location.Title
				}

				norm := engine.NormalizeSubject(p.Name)
				tag := engine.NormalizeTag(p.Tag)
				url := ""
				if groupURLs != nil {
					url = groupURLs[norm+"|"+tag]
				}

				views = append(views, lessonView{
					Time:        p.Time,
					Name:        p.Name,
					Tag:         p.Tag,
					TeacherRaw:  teacherRaw,
					LocationRaw: locRaw,
					Lecturer:    lView,
					Location:    locView,
					Enriched:    true,
					URL:         url,
				})
			}
			sort.Slice(views, func(i, j int) bool { return views[i].Time < views[j].Time })
			days = append(days, weekDayView{
				Day:     short,
				DayName: dayNamesUA[dayIdx],
				Lessons: views,
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

// GetUniqueGroupLessons returns deduplicated lessons from the group's Campus API schedule,
// populated with existing custom URLs from bot_group_lesson_urls.
func (s *Service) GetUniqueGroupLessons(ctx context.Context, botGroupID uuid.UUID, academicGroupID int) ([]model.UniqueLesson, error) {
	sched, err := s.campus.GroupSchedule(ctx, academicGroupID)
	if err != nil {
		return nil, fmt.Errorf("fetching group schedule: %w", err)
	}

	urls, err := s.db.GetGroupLessonURLs(ctx, botGroupID)
	if err != nil {
		return nil, fmt.Errorf("fetching group lesson urls: %w", err)
	}

	type groupData struct {
		subject     string
		subjectNorm string
		tag         string
		hasOnline   bool
	}
	groups := make(map[string]*groupData)
	var groupKeys []string

	scanPairs := func(days []campus.DaySchedule) {
		for _, d := range days {
			for _, p := range d.Pairs {
				tag := engine.NormalizeTag(p.Tag)
				norm := engine.NormalizeSubject(p.Name)
				key := norm + "|" + tag

				loc := ""
				if p.Location != nil {
					loc = p.Location.Title
				}
				isOnline := model.IsOnline(loc)

				g, exists := groups[key]
				if !exists {
					g = &groupData{
						subject:     p.Name,
						subjectNorm: norm,
						tag:         tag,
						hasOnline:   isOnline,
					}
					groups[key] = g
					groupKeys = append(groupKeys, key)
				} else {
					if isOnline {
						g.hasOnline = true
					}
					if g.subject == "" && p.Name != "" {
						g.subject = p.Name
					}
				}
			}
		}
	}

	scanPairs(sched.ScheduleFirstWeek)
	scanPairs(sched.ScheduleSecondWeek)

	var unique []model.UniqueLesson
	for _, key := range groupKeys {
		g := groups[key]
		if !g.hasOnline {
			continue
		}
		unique = append(unique, model.UniqueLesson{
			Subject:     g.subject,
			SubjectNorm: g.subjectNorm,
			Tag:         g.tag,
			IsOnline:    true,
			URL:         urls[key],
		})
	}

	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Subject != unique[j].Subject {
			return unique[i].Subject < unique[j].Subject
		}
		return unique[i].Tag < unique[j].Tag
	})

	return unique, nil
}
