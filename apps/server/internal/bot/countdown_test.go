package bot

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d        time.Duration
		expected string
	}{
		{0, "00:00:00"},
		{-5 * time.Second, "00:00:00"},
		{51 * time.Second, "00:00:51"},
		{50*time.Second + 100*time.Millisecond, "00:00:51"}, // Ceil rounding
		{1*time.Hour + 28*time.Minute + 26*time.Second, "01:28:26"},
		{2*time.Hour + 5*time.Minute + 3*time.Second, "02:05:03"},
	}

	for _, tc := range cases {
		got := formatDuration(tc.d)
		if got != tc.expected {
			t.Errorf("formatDuration(%v) = %q; want %q", tc.d, got, tc.expected)
		}
	}
}

func TestParseClock(t *testing.T) {
	h, m, s, ok := parseClock("08:30:00")
	if !ok || h != 8 || m != 30 || s != 0 {
		t.Fatalf("parseClock('08:30:00') = (%d, %d, %d, %v)", h, m, s, ok)
	}

	h, m, s, ok = parseClock("14:15")
	if !ok || h != 14 || m != 15 || s != 0 {
		t.Fatalf("parseClock('14:15') = (%d, %d, %d, %v)", h, m, s, ok)
	}

	_, _, _, ok = parseClock("invalid")
	if ok {
		t.Fatalf("parseClock('invalid') should fail")
	}
}

func TestFormatCountdown(t *testing.T) {
	loc := kyivLocation()
	targetDate := "2026-09-03"

	lessons := []lessonLine{
		{
			Time:    "08:30:00",
			EndTime: "10:05:00",
			Name:    "Компоненти програмної інженерії",
			Tag:     "lec",
		},
		{
			Time:    "10:25:00",
			EndTime: "12:00:00",
			Name:    "Архітектура та проєктування",
			Tag:     "lec",
		},
		{
			Time:    "12:20:00",
			EndTime: "13:55:00",
			Name:    "Програмування мікроконтролерів",
			Tag:     "lec",
		},
		{
			Time:    "14:15:00",
			EndTime: "15:50:00",
			Name:    "Основи Інтернету речей",
			Tag:     "prac",
		},
		{
			Time:    "16:10:00",
			EndTime: "17:45:00",
			Name:    "Програмне забезпечення мереж",
			Tag:     "prac",
		},
	}

	// 1. Morning before first lesson (08:00:00) -> countdown to 08:30
	t.Run("MorningBeforeFirstLesson", func(t *testing.T) {
		day := dayInfo{
			Date:    targetDate,
			DayName: "Четвер",
			Lessons: lessons,
			Now:     time.Date(2026, 9, 3, 8, 0, 0, 0, loc),
		}
		got := formatCountdown(day)
		want := "<blockquote>До кінця перерви лишилося: <b>00:30:00</b></blockquote>"
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	})

	// 2. During first lesson (09:00:00) -> countdown to 10:05
	t.Run("DuringFirstLesson", func(t *testing.T) {
		day := dayInfo{
			Date:    targetDate,
			DayName: "Четвер",
			Lessons: lessons,
			Now:     time.Date(2026, 9, 3, 9, 0, 0, 0, loc),
		}
		got := formatCountdown(day)
		want := "<blockquote>До кінця пари лишилося: <b>01:05:00</b></blockquote>"
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	})

	// 3. During break matching Screenshot 2: at 14:14:09 -> 51s to 14:15:00
	t.Run("BreakMatchingScreenshot2", func(t *testing.T) {
		day := dayInfo{
			Date:    targetDate,
			DayName: "Четвер",
			Lessons: lessons,
			Now:     time.Date(2026, 9, 3, 14, 14, 9, 0, loc),
		}
		got := formatCountdown(day)
		want := "<blockquote>До кінця перерви лишилося: <b>00:00:51</b></blockquote>"
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	})

	// 4. During lesson matching Screenshot 1: at 16:16:34 -> 1h 28m 26s to 17:45:00
	t.Run("LessonMatchingScreenshot1", func(t *testing.T) {
		day := dayInfo{
			Date:    targetDate,
			DayName: "Четвер",
			Lessons: lessons,
			Now:     time.Date(2026, 9, 3, 16, 16, 34, 0, loc),
		}
		got := formatCountdown(day)
		want := "<blockquote>До кінця пари лишилося: <b>01:28:26</b></blockquote>"
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	})

	// 5. Break between lessons (10:15:00) -> 10m to 10:25:00
	t.Run("InterLessonBreak", func(t *testing.T) {
		day := dayInfo{
			Date:    targetDate,
			DayName: "Четвер",
			Lessons: lessons,
			Now:     time.Date(2026, 9, 3, 10, 15, 0, 0, loc),
		}
		got := formatCountdown(day)
		want := "<blockquote>До кінця перерви лишилося: <b>00:10:00</b></blockquote>"
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	})

	// 6. After all lessons ended (17:45:01) -> nothing shown
	t.Run("AfterAllLessonsEnded", func(t *testing.T) {
		day := dayInfo{
			Date:    targetDate,
			DayName: "Четвер",
			Lessons: lessons,
			Now:     time.Date(2026, 9, 3, 17, 45, 1, 0, loc),
		}
		got := formatCountdown(day)
		if got != "" {
			t.Errorf("got %q; want empty string", got)
		}
	})

	// 7. Day off -> nothing shown
	t.Run("DayOff", func(t *testing.T) {
		day := dayInfo{
			Date:     targetDate,
			DayName:  "Неділя",
			IsDayOff: true,
			Lessons:  nil,
			Now:      time.Date(2026, 9, 3, 12, 0, 0, 0, loc),
		}
		got := formatCountdown(day)
		if got != "" {
			t.Errorf("got %q; want empty string", got)
		}
	})

	// 8. Viewing a different day (e.g. tomorrow 2026-09-04) -> nothing shown
	t.Run("ViewingDifferentDate", func(t *testing.T) {
		day := dayInfo{
			Date:    "2026-09-04",
			DayName: "П'ятниця",
			Lessons: lessons,
			Now:     time.Date(2026, 9, 3, 10, 0, 0, 0, loc),
		}
		got := formatCountdown(day)
		if got != "" {
			t.Errorf("got %q; want empty string", got)
		}
	})

	// 9. Defaulting duration to 95 minutes when EndTime is empty
	t.Run("EmptyEndTimeDefaultsTo95Minutes", func(t *testing.T) {
		singleLesson := []lessonLine{
			{
				Time: "12:20:00",
				Name: "Програмування",
				Tag:  "lec",
			},
		}
		// At 12:50:00, remaining to 13:55:00 is 1h 05m
		day := dayInfo{
			Date:    targetDate,
			DayName: "Четвер",
			Lessons: singleLesson,
			Now:     time.Date(2026, 9, 3, 12, 50, 0, 0, loc),
		}
		got := formatCountdown(day)
		want := "<blockquote>До кінця пари лишилося: <b>01:05:00</b></blockquote>"
		if got != want {
			t.Errorf("got %q; want %q", got, want)
		}
	})
}

func TestFormatDayWithCountdown(t *testing.T) {
	loc := kyivLocation()
	targetDate := "2026-09-03"

	day := dayInfo{
		Date:    targetDate,
		DayName: "Четвер",
		Lessons: []lessonLine{
			{Time: "16:10:00", EndTime: "17:45:00", Name: "Програмне забезпечення", Tag: "prac", LocationRaw: "Онлайн"},
		},
		Now: time.Date(2026, 9, 3, 16, 16, 34, 0, loc),
	}

	res := formatDay(day)
	if !strings.Contains(res, "<blockquote>До кінця пари лишилося: <b>01:28:26</b></blockquote>") {
		t.Fatalf("expected lesson countdown blockquote in formatDay output, got:\n%s", res)
	}

	// Group day
	groupRes := formatGroupDay(day, "ІП-21")
	if !strings.Contains(groupRes, "<blockquote>До кінця пари лишилося: <b>01:28:26</b></blockquote>") {
		t.Fatalf("expected lesson countdown blockquote in formatGroupDay output, got:\n%s", groupRes)
	}

	// When lessons ended, no blockquote
	day.Now = time.Date(2026, 9, 3, 18, 0, 0, 0, loc)
	resEnded := formatDay(day)
	if strings.Contains(resEnded, "<blockquote>") {
		t.Fatalf("did not expect blockquote after lessons ended, got:\n%s", resEnded)
	}
}
