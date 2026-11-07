package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"kpi-schedule-bot/server/internal/model"
)

// recordingBotClient stands in for the Telegram Bot API: it records every
// method call and answers each one with `true`, which is enough for
// answerCallbackQuery.
type recordingBotClient struct {
	calls []recordedCall
}

type recordedCall struct {
	method string
	params map[string]any
}

func (c *recordingBotClient) RequestWithContext(_ context.Context, _ string, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	c.calls = append(c.calls, recordedCall{method: method, params: params})
	return json.RawMessage("true"), nil
}

func (c *recordingBotClient) GetAPIURL(*gotgbot.RequestOpts) string { return "" }

func (c *recordingBotClient) FileURL(string, string, *gotgbot.RequestOpts) string { return "" }

func newRecordingTGBot(t *testing.T) (*gotgbot.Bot, *recordingBotClient) {
	t.Helper()
	client := &recordingBotClient{}
	tg, err := gotgbot.NewBot("123456789:AAFakeTokenForTestingScheduleLock", &gotgbot.BotOpts{
		BotClient:         client,
		DisableTokenCheck: true,
	})
	if err != nil {
		t.Fatalf("creating recording bot: %v", err)
	}
	return tg, client
}

func groupCallback(fromID int64, data string) *gotgbot.CallbackQuery {
	return &gotgbot.CallbackQuery{
		Id:   "cq-1",
		From: gotgbot.User{Id: fromID, FirstName: "Тест"},
		Data: data,
		Message: &gotgbot.Message{
			MessageId: 42,
			Chat:      gotgbot.Chat{Id: -100123, Type: "supergroup"},
		},
	}
}

func TestPersonalScheduleCallbackDataCarriesOwner(t *testing.T) {
	const owner int64 = 7012345678
	date := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	day := dayKeyboard(date, owner)
	for _, btn := range day.InlineKeyboard[0] {
		if !strings.HasSuffix(btn.CallbackData, ":7012345678") {
			t.Errorf("day button %q lacks owner suffix: %q", btn.Text, btn.CallbackData)
		}
		if len(btn.CallbackData) > 64 {
			t.Errorf("callback data exceeds 64 bytes: %q", btn.CallbackData)
		}
	}

	week := weekKeyboard(0, owner)
	for _, row := range week.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == weekNoopCallbackData() {
				continue
			}
			if !strings.HasSuffix(btn.CallbackData, ":7012345678") {
				t.Errorf("week button %q lacks owner suffix: %q", btn.Text, btn.CallbackData)
			}
		}
	}
	if got := week.InlineKeyboard[0][0].CallbackData; got != "week:goto:-1:7012345678" {
		t.Errorf("unexpected previous-week callback data %q", got)
	}
	if got := week.InlineKeyboard[1][0].CallbackData; got != "week:today:7012345678" {
		t.Errorf("unexpected week-today callback data %q", got)
	}
}

func TestParseOwnerID(t *testing.T) {
	cases := []struct {
		raw    string
		wantID int64
		wantOK bool
	}{
		{"", 0, true}, // legacy callback without owner segment
		{"7012345678", 7012345678, true},
		{"abc", 0, false},
	}
	for _, c := range cases {
		id, ok := parseOwnerID(c.raw)
		if id != c.wantID || ok != c.wantOK {
			t.Errorf("parseOwnerID(%q) = (%d, %v), want (%d, %v)", c.raw, id, ok, c.wantID, c.wantOK)
		}
	}
}

func TestAllowPersonalScheduleNav(t *testing.T) {
	ctx := context.Background()
	b, db, _ := setupTestBot(t)

	const (
		lockedOwner   int64 = 1001
		unlockedOwner int64 = 1002
		stranger      int64 = 2002
	)
	if _, err := db.UpsertUser(ctx, lockedOwner, nil, nil); err != nil {
		t.Fatalf("creating locked owner: %v", err)
	}
	if err := db.SetUserScheduleLock(ctx, lockedOwner, true); err != nil {
		t.Fatalf("locking owner schedule: %v", err)
	}
	if _, err := db.UpsertUser(ctx, unlockedOwner, nil, nil); err != nil {
		t.Fatalf("creating unlocked owner: %v", err)
	}

	cases := []struct {
		name      string
		caller    int64
		owner     int64
		wantAllow bool
	}{
		{"owner presses own locked schedule", lockedOwner, lockedOwner, true},
		{"stranger presses locked schedule", stranger, lockedOwner, false},
		{"stranger presses unlocked schedule", stranger, unlockedOwner, true},
		{"legacy message without owner", stranger, 0, true},
		{"owner unknown to the bot", stranger, 9999, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tg, client := newRecordingTGBot(t)
			allowed, err := b.allowPersonalScheduleNav(tg, groupCallback(c.caller, ""), c.owner)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if allowed != c.wantAllow {
				t.Fatalf("allowed = %v, want %v", allowed, c.wantAllow)
			}
			if c.wantAllow {
				if len(client.calls) != 0 {
					t.Errorf("allowed press must not answer the callback itself, got %+v", client.calls)
				}
				return
			}
			if len(client.calls) != 1 || client.calls[0].method != "answerCallbackQuery" {
				t.Fatalf("expected a single answerCallbackQuery, got %+v", client.calls)
			}
			if client.calls[0].params["show_alert"] != true {
				t.Errorf("expected a popup alert, got %+v", client.calls[0].params)
			}
			if client.calls[0].params["text"] != scheduleLockedAlertText {
				t.Errorf("unexpected alert text %v", client.calls[0].params["text"])
			}
		})
	}
}

// TestLockedScheduleButtonsLeaveMessageUntouched drives the real nav/week
// handlers: a stranger's press on a locked owner's message must only get the
// popup — no editMessageText that would swap in the stranger's schedule.
func TestLockedScheduleButtonsLeaveMessageUntouched(t *testing.T) {
	ctx := context.Background()
	b, db, _ := setupTestBot(t)

	const owner, stranger int64 = 1001, 2002
	if _, err := db.UpsertUser(ctx, owner, nil, nil); err != nil {
		t.Fatalf("creating owner: %v", err)
	}
	if err := db.SetUserScheduleLock(ctx, owner, true); err != nil {
		t.Fatalf("locking owner schedule: %v", err)
	}

	date := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	presses := []struct {
		data    string
		handler func(*gotgbot.Bot, *ext.Context) error
	}{
		{navCallbackData("next", date, owner), b.onNav},
		{navCallbackData("today", date, owner), b.onNav},
		{weekCallbackData(1, owner), b.onWeek},
		{weekTodayCallbackData(owner), b.onWeek},
	}
	for _, p := range presses {
		t.Run(p.data, func(t *testing.T) {
			tg, client := newRecordingTGBot(t)
			cq := groupCallback(stranger, p.data)
			if err := p.handler(tg, &ext.Context{Update: &gotgbot.Update{CallbackQuery: cq}}); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			for _, call := range client.calls {
				if call.method != "answerCallbackQuery" {
					t.Errorf("locked press must not touch the message, got %s", call.method)
				}
			}
			if len(client.calls) != 1 || client.calls[0].params["text"] != scheduleLockedAlertText {
				t.Errorf("expected the lock popup, got %+v", client.calls)
			}
		})
	}
}

func TestUserSettingsScreenShowsScheduleLock(t *testing.T) {
	off := model.User{NotificationsEnabled: true}
	if text := formatUserSettings(off); !strings.Contains(text, "Захист персонального розкладу:</b> 🔓 Вимкнено") {
		t.Errorf("expected lock shown as off, got:\n%s", text)
	}
	kb := userSettingsKeyboard(off)
	if len(kb.InlineKeyboard) != 3 {
		t.Fatalf("expected notify, lock and back rows, got %d rows", len(kb.InlineKeyboard))
	}
	if btn := kb.InlineKeyboard[1][0]; btn.CallbackData != "settings:toggle_lock" || btn.Text != "🔓 Захист розкладу: Вимкнено" {
		t.Errorf("unexpected lock button %+v", btn)
	}

	on := model.User{ScheduleLocked: true}
	if text := formatUserSettings(on); !strings.Contains(text, "Захист персонального розкладу:</b> 🔒 Увімкнено") {
		t.Errorf("expected lock shown as on, got:\n%s", text)
	}
	if btn := userSettingsKeyboard(on).InlineKeyboard[1][0]; btn.Text != "🔒 Захист розкладу: Увімкнено" {
		t.Errorf("unexpected lock button text %q", btn.Text)
	}
}
