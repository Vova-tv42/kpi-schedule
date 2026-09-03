package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

func TestFormatUserName(t *testing.T) {
	cases := []struct {
		user     *gotgbot.User
		expected string
	}{
		{
			user:     nil,
			expected: "користувача",
		},
		{
			user: &gotgbot.User{
				Id:        123,
				FirstName: "Іван",
				LastName:  "Франко",
				Username:  "franko",
			},
			expected: "Іван Франко (@franko)",
		},
		{
			user: &gotgbot.User{
				Id:        456,
				FirstName: "Леся",
				LastName:  "",
			},
			expected: "Леся",
		},
		{
			user: &gotgbot.User{
				Id:       789,
				Username: "taras",
			},
			expected: "@taras",
		},
		{
			user: &gotgbot.User{
				Id: 999,
			},
			expected: "ID:999",
		},
	}

	for _, tc := range cases {
		res := formatUserName(tc.user)
		if res != tc.expected {
			t.Errorf("formatUserName(%v) = %q, want %q", tc.user, res, tc.expected)
		}
	}
}

func TestFormatDayCallerName(t *testing.T) {
	day := dayInfo{
		Date:       "2026-09-02",
		DayName:    "Вівторок",
		CallerName: "Тарас Шевченко",
		Lessons: []lessonLine{
			{Time: "08:30:00", Name: "Тестування ПЗ", Tag: "lec", LocationRaw: "Онлайн"},
		},
	}

	res := formatDay(day)
	if !strings.Contains(res, "👤 Розклад: <b>Тарас Шевченко</b>") {
		t.Fatalf("expected caller name in output, got: %s", res)
	}

	// Without caller name
	day.CallerName = ""
	resWithout := formatDay(day)
	if strings.Contains(resWithout, "👤 Розклад:") {
		t.Fatalf("did not expect caller name in DM output, got: %s", resWithout)
	}
}

func TestFormatWeekCallerName(t *testing.T) {
	week := weekInfo{
		WeekNumber: 1,
		CallerName: "Леся Українка",
		Days: []weekDayLine{
			{
				DayName: "Понеділок",
				Lessons: []lessonLine{
					{Time: "10:25:00", Name: "Бази даних", Tag: "prac", LocationRaw: "Онлайн"},
				},
			},
		},
	}

	groupName := "ІП-21"
	res := formatWeek(week, 0, &groupName)
	if !strings.Contains(res, "👤 Розклад: <b>Леся Українка</b>") {
		t.Fatalf("expected caller name in week output, got: %s", res)
	}

	week.CallerName = ""
	resWithout := formatWeek(week, 0, &groupName)
	if strings.Contains(resWithout, "👤 Розклад:") {
		t.Fatalf("did not expect caller name in DM week output, got: %s", resWithout)
	}
}

func TestFormatGroupDayAndKeyboard(t *testing.T) {
	day := dayInfo{
		Date:    "2026-09-02",
		DayName: "Вівторок",
		Lessons: []lessonLine{
			{
				Time:        "08:30:00",
				Name:        "Основи AI",
				Tag:         "lec",
				TeacherRaw:  "Проф. Бондар",
				LocationRaw: "5-306",
			},
		},
	}

	res := formatGroupDay(day, "ІП-21")
	if !strings.Contains(res, "👥 <b>Розклад групи ІП-21</b>") {
		t.Fatalf("expected group header, got: %s", res)
	}
	if !strings.Contains(res, "Основи AI") || !strings.Contains(res, "Проф. Бондар") {
		t.Fatalf("missing lesson info: %s", res)
	}

	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	kb := groupDayKeyboard(now, 4402)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 3 {
		t.Fatalf("unexpected keyboard shape: %+v", kb)
	}
	if !strings.Contains(kb.InlineKeyboard[0][0].CallbackData, "gnav:prev:") {
		t.Errorf("expected gnav:prev, got %s", kb.InlineKeyboard[0][0].CallbackData)
	}
}

func TestFormatGroupWeekAndKeyboard(t *testing.T) {
	week := weekInfo{
		WeekNumber: 2,
		Days: []weekDayLine{
			{
				DayName: "Середа",
				Lessons: []lessonLine{
					{Time: "12:20:00", Name: "Архітектура ПЗ", Tag: "lab", LocationRaw: "Онлайн"},
				},
			},
		},
	}

	res := formatGroupWeek(week, 1, "ІП-21")
	if !strings.Contains(res, "👥 <b>Розклад групи ІП-21</b>") {
		t.Fatalf("expected group header, got: %s", res)
	}
	if !strings.Contains(res, "Другий</b> тиждень") {
		t.Fatalf("expected week number, got: %s", res)
	}

	kb := groupWeekKeyboard(1, 4402)
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in week keyboard, got: %d", len(kb.InlineKeyboard))
	}
	// Row 1: prev, current, next
	if !strings.Contains(kb.InlineKeyboard[0][2].Text, "✅") {
		t.Errorf("expected next slot (offset 1) marked active: %+v", kb.InlineKeyboard[0][2])
	}
}

func TestGroupListAndConfigFormatting(t *testing.T) {
	gid := uuid.New()
	chatID := int64(-100123456789)
	groups := []model.BotGroup{
		{
			ID:                gid,
			AcademicGroupID:   4402,
			AcademicGroupName: "ІП-21",
			Faculty:           "ФІОТ",
			TelegramChatID:    &chatID,
			TelegramChatTitle: "Чат групи ІП-21",
		},
	}

	// List menu
	listText := formatGroupListMenu(groups, "Дія успішна")
	if !strings.Contains(listText, "Керування групами") || !strings.Contains(listText, "Дія успішна") {
		t.Fatalf("unexpected list menu text: %s", listText)
	}

	listKb := groupListKeyboard(groups)
	if len(listKb.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows (group + new), got: %d", len(listKb.InlineKeyboard))
	}

	// Config screen
	cfgText := formatGroupConfig(groups[0], "")
	if !strings.Contains(cfgText, "ІП-21 (ФІОТ)") || !strings.Contains(cfgText, "Чат групи ІП-21") {
		t.Fatalf("unexpected config text: %s", cfgText)
	}

	cfgKb := groupConfigKeyboard(groups[0])
	if len(cfgKb.InlineKeyboard) < 4 {
		t.Fatalf("expected at least 4 config buttons, got: %d", len(cfgKb.InlineKeyboard))
	}

	// Delete confirm
	delText := formatGroupDeleteConfirm(groups[0])
	if !strings.Contains(delText, "Видалення групи") || !strings.Contains(delText, "ІП-21") {
		t.Fatalf("unexpected delete text: %s", delText)
	}

	delKb := groupDeleteConfirmKeyboard(gid.String())
	if len(delKb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 2 buttons in delete confirm: %+v", delKb)
	}
}

func TestIsGroupChat(t *testing.T) {
	if isGroupChat(nil) {
		t.Errorf("nil chat should not be group")
	}
	if isGroupChat(&gotgbot.Chat{Type: "private"}) {
		t.Errorf("private chat should not be group")
	}
	if !isGroupChat(&gotgbot.Chat{Type: "group"}) {
		t.Errorf("group chat should be group")
	}
	if !isGroupChat(&gotgbot.Chat{Type: "supergroup"}) {
		t.Errorf("supergroup chat should be group")
	}
}
