package bot

import (
	"strings"
	"testing"

	"kpi-schedule-bot/server/internal/model"
)

func TestIsValidURL(t *testing.T) {
	validCases := []string{
		"https://zoom.us/j/123456789",
		"http://meet.google.com/abc-defg-hij",
		"https://teams.microsoft.com/l/meetup-join/19%3ameeting",
		"https://sub.domain.kpi.ua/path?query=1&b=2#frag",
	}
	for _, c := range validCases {
		if !isValidURL(c) {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalidCases := []string{
		"",
		"   ",
		"zoom.us/123", // missing scheme
		"ftp://files.example.com", // unsupported scheme
		"javascript:alert(1)",
		"http://",
		"https://",
		"http://nodot",
		"http:///path",
		"hello world",
	}
	for _, c := range invalidCases {
		if isValidURL(c) {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}

func TestFormatLessonMode(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		location string
		url      string
		expected string
	}{
		{
			name:     "online lecture without url",
			tag:      "lec",
			location: "Онлайн Zoom",
			url:      "",
			expected: "[Лек., Онлайн]",
		},
		{
			name:     "online practice with url",
			tag:      "prac",
			location: "Онлайн Meet",
			url:      "https://meet.google.com/abc-defg",
			expected: `<a href="https://meet.google.com/abc-defg">[Практ., Онлайн]</a>`,
		},
		{
			name:     "offline practice",
			tag:      "prac",
			location: "18-402",
			url:      "",
			expected: "[Практ., Оффлайн]",
		},
		{
			name:     "lab without location defaults to online",
			tag:      "lab",
			location: "",
			url:      "",
			expected: "[Лаб., Онлайн]",
		},
		{
			name:     "url with special characters escaped",
			tag:      "lec",
			location: "Онлайн",
			url:      "https://zoom.us/j/123?a=1&b=2",
			expected: `<a href="https://zoom.us/j/123?a=1&amp;b=2">[Лек., Онлайн]</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLessonMode(tt.tag, tt.location, tt.url)
			if got != tt.expected {
				t.Errorf("got %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestLessonHash(t *testing.T) {
	h1 := lessonHash("технології devops", "lec")
	h2 := lessonHash("технології devops", "lec")
	h3 := lessonHash("технології devops", "prac")

	if len(h1) != 12 {
		t.Errorf("expected 12 chars, got %d (%q)", len(h1), h1)
	}
	if h1 != h2 {
		t.Errorf("expected deterministic hash: %q vs %q", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("expected different hash for different tag: %q vs %q", h1, h3)
	}
}

func TestFormatDayWithURLs(t *testing.T) {
	info := dayInfo{
		Date:    "2026-09-01",
		DayName: "Вівторок",
		Lessons: []lessonLine{
			{
				Time:          "08:30:00",
				Name:          "Процеси розробки ПЗ",
				Tag:           "lec",
				LocationTitle: "18-402",
			},
			{
				Time:        "10:25:00",
				Name:        "Технології DevOps",
				Tag:         "prac",
				LocationRaw: "Онлайн Zoom",
				URL:         "https://zoom.us/j/999",
			},
		},
	}

	out := formatDay(info)

	// Offline class without URL
	if !strings.Contains(out, "[Лек., Оффлайн]") {
		t.Errorf("expected offline badge [Лек., Оффлайн] in output, got:\n%s", out)
	}

	// Online class with URL
	expectedLink := `<a href="https://zoom.us/j/999">[Практ., Онлайн]</a>`
	if !strings.Contains(out, expectedLink) {
		t.Errorf("expected clickable link %s in output, got:\n%s", expectedLink, out)
	}
}

func TestFormatWeekWithURLs(t *testing.T) {
	group := "ТВ-42"
	info := weekInfo{
		WeekNumber: 1,
		Days: []weekDayLine{
			{
				DayName: "Понеділок",
				Lessons: []lessonLine{
					{
						Time:        "10:25:00",
						Name:        "Технології DevOps",
						Tag:         "lec",
						LocationRaw: "Онлайн Zoom",
						URL:         "https://zoom.us/j/123",
					},
				},
			},
		},
	}

	out := formatWeek(info, 0, &group)
	expectedLink := `<a href="https://zoom.us/j/123">[Лек., Онлайн]</a>`
	if !strings.Contains(out, expectedLink) {
		t.Errorf("expected clickable link %s in week view, got:\n%s", expectedLink, out)
	}
}

func TestFormatLessonsMenu(t *testing.T) {
	lessons := []model.UniqueLesson{
		{
			Subject:     "Технології DevOps",
			SubjectNorm: "технології devops",
			Tag:         "lec",
			IsOnline:    true,
			URL:         "https://zoom.us/j/123",
		},
		{
			Subject:     "Технології DevOps",
			SubjectNorm: "технології devops",
			Tag:         "prac",
			IsOnline:    true,
			URL:         "",
		},
	}

	out := formatLessonsMenu(lessons, "✅ Збережено!")
	if !strings.Contains(out, "✅ Збережено!") {
		t.Errorf("expected notice in menu output:\n%s", out)
	}
	if !strings.Contains(out, `<a href="https://zoom.us/j/123">[Лек., Онлайн]</a>`) {
		t.Errorf("expected link for lecture in menu output:\n%s", out)
	}
	if !strings.Contains(out, "[Практ., Онлайн]") {
		t.Errorf("expected plain badge for unlinked practice in menu output:\n%s", out)
	}
}
