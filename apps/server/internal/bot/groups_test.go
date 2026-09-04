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
	if kb.InlineKeyboard[0][0].Text != "◀️" {
		t.Errorf("expected prev button text '◀️', got %q", kb.InlineKeyboard[0][0].Text)
	}
	if kb.InlineKeyboard[0][1].Text != "📅 Сьогодні" {
		t.Errorf("expected today button text '📅 Сьогодні', got %q", kb.InlineKeyboard[0][1].Text)
	}
	if kb.InlineKeyboard[0][2].Text != "▶️" {
		t.Errorf("expected next button text '▶️', got %q", kb.InlineKeyboard[0][2].Text)
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

	// Config screen as creator
	cfgText := formatGroupConfig(groups[0], "", true)
	if !strings.Contains(cfgText, "ІП-21 (ФІОТ)") || !strings.Contains(cfgText, "Чат групи ІП-21") || !strings.Contains(cfgText, "👑 Творець") {
		t.Fatalf("unexpected config text for creator: %s", cfgText)
	}

	cfgKb := groupConfigKeyboard(groups[0], true)
	if len(cfgKb.InlineKeyboard) < 5 {
		t.Fatalf("expected at least 5 config buttons for creator, got: %d", len(cfgKb.InlineKeyboard))
	}

	// Config screen as co-admin
	cfgCoText := formatGroupConfig(groups[0], "", false)
	if !strings.Contains(cfgCoText, "👤 Адміністратор") {
		t.Fatalf("expected co-admin status in text: %s", cfgCoText)
	}
	cfgCoKb := groupConfigKeyboard(groups[0], false)
	foundLeave := false
	for _, row := range cfgCoKb.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Вийти з керування") {
				foundLeave = true
			}
		}
	}
	if !foundLeave {
		t.Errorf("expected 'Вийти з керування' button for co-admin")
	}

	// Delete confirm for creator with no other admins
	delText := formatGroupDeleteConfirm(groups[0], true, false)
	if !strings.Contains(delText, "Повне видалення групи") || !strings.Contains(delText, "ІП-21") {
		t.Fatalf("unexpected delete text: %s", delText)
	}

	// Delete confirm for creator with other admins
	delTransferText := formatGroupDeleteConfirm(groups[0], true, true)
	if !strings.Contains(delTransferText, "передано одному з них") {
		t.Fatalf("unexpected transfer text: %s", delTransferText)
	}

	// Delete confirm for co-admin
	delCoText := formatGroupDeleteConfirm(groups[0], false, false)
	if !strings.Contains(delCoText, "Вихід з керування групою") {
		t.Fatalf("unexpected co-admin delete text: %s", delCoText)
	}

	delKb := groupDeleteConfirmKeyboard(gid.String())
	if len(delKb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 2 buttons in delete confirm: %+v", delKb)
	}

	// Admins list formatting
	admins := []model.GroupAdmin{
		{
			GroupID:    gid,
			TelegramID: 111,
			FirstName:  "Тарас",
			Username:   "sheva",
			Status:     model.GroupAdminAccepted,
		},
		{
			GroupID:    gid,
			TelegramID: 222,
			FirstName:  "Леся",
			Username:   "",
			Status:     model.GroupAdminInvited,
		},
	}
	adminsText := formatGroupAdmins(groups[0], admins, "")
	if !strings.Contains(adminsText, "Тарас (@sheva)") || !strings.Contains(adminsText, "Леся") {
		t.Fatalf("missing admin in formatGroupAdmins: %s", adminsText)
	}
	admKb := groupAdminsKeyboard(gid.String(), admins, true)
	if len(admKb.InlineKeyboard) != 4 { // 2 admins + add + back
		t.Fatalf("expected 4 rows in groupAdminsKeyboard, got: %d", len(admKb.InlineKeyboard))
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
