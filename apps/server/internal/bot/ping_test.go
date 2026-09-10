package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

func TestParseUsernames(t *testing.T) {
	input := "@alice, @bob_99   @test_bot\n@charlie, @Alice @d @bad-name @super_long_username_that_is_way_too_long_for_telegram_rules"
	valid, bots, invalid := parseUsernames(input)

	// Valid should contain alice, bob_99, charlie (deduplicated case-insensitively)
	if len(valid) != 3 {
		t.Fatalf("expected 3 valid usernames, got %d: %+v", len(valid), valid)
	}
	expectedValid := []string{"alice", "bob_99", "charlie"}
	for i, exp := range expectedValid {
		if strings.ToLower(valid[i]) != exp {
			t.Errorf("at index %d: expected %s, got %s", i, exp, valid[i])
		}
	}

	// Bots should contain test_bot
	if len(bots) != 1 || bots[0] != "test_bot" {
		t.Errorf("expected bots [test_bot], got %+v", bots)
	}

	// Invalid should contain d, bad-name, super_long_...
	if len(invalid) != 3 {
		t.Errorf("expected 3 invalid usernames, got %d: %+v", len(invalid), invalid)
	}
}

func TestGroupPingMenuAndKeyboard(t *testing.T) {
	groupID := uuid.New().String()

	// 1. Empty users
	emptyMenu := formatGroupPingMenu("ІП-21", nil, "")
	if !strings.Contains(emptyMenu, "Список користувачів для /ping порожній") {
		t.Errorf("expected empty notice, got:\n%s", emptyMenu)
	}
	emptyKB := groupPingKeyboard(groupID, nil, 0)
	// Should have [➕ Додати користувачів] and [◀️ Назад до налаштувань] (2 rows, no pagination)
	if len(emptyKB.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows in empty keyboard, got %d", len(emptyKB.InlineKeyboard))
	}

	// 2. 3 users (<= 5: no pagination row)
	users3 := []string{"user1", "user2", "user3"}
	menu3 := formatGroupPingMenu("ІП-21", users3, "")
	if !strings.Contains(menu3, "@user1 @user2 @user3") {
		t.Errorf("expected usernames in quote block, got:\n%s", menu3)
	}
	kb3 := groupPingKeyboard(groupID, users3, 0)
	// 3 user buttons + 1 add button + 1 back button = 5 rows, no pagination
	if len(kb3.InlineKeyboard) != 5 {
		t.Fatalf("expected 5 rows for 3 users without pagination, got %d", len(kb3.InlineKeyboard))
	}

	// 3. 12 users (12 users / 5 per page = 3 pages: 0, 1, 2)
	users12 := make([]string, 12)
	for i := 0; i < 12; i++ {
		users12[i] = "student" + string(rune('a'+i))
	}

	// Page 0
	kbPage0 := groupPingKeyboard(groupID, users12, 0)
	// 5 user buttons + 1 pagination row + 1 add button + 1 back button = 8 rows
	if len(kbPage0.InlineKeyboard) != 8 {
		t.Fatalf("expected 8 rows on page 0, got %d", len(kbPage0.InlineKeyboard))
	}
	pagRow0 := kbPage0.InlineKeyboard[5]
	if len(pagRow0) != 2 {
		t.Fatalf("expected 2 buttons in pagination row, got %d", len(pagRow0))
	}
	if pagRow0[0].Text != "⏹️" || pagRow0[0].CallbackData != groupCallbackPrefix+"ping_noop" {
		t.Errorf("expected disabled prev button on page 0, got %+v", pagRow0[0])
	}
	if pagRow0[1].Text != "▶️" || !strings.Contains(pagRow0[1].CallbackData, "ping_page:"+groupID+":1") {
		t.Errorf("expected next button to page 1, got %+v", pagRow0[1])
	}

	// Page 1 (middle page)
	kbPage1 := groupPingKeyboard(groupID, users12, 1)
	pagRow1 := kbPage1.InlineKeyboard[5]
	if pagRow1[0].Text != "◀️" || !strings.Contains(pagRow1[0].CallbackData, "ping_page:"+groupID+":0") {
		t.Errorf("expected prev button to page 0, got %+v", pagRow1[0])
	}
	if pagRow1[1].Text != "▶️" || !strings.Contains(pagRow1[1].CallbackData, "ping_page:"+groupID+":2") {
		t.Errorf("expected next button to page 2, got %+v", pagRow1[1])
	}

	// Page 2 (last page, 2 users)
	kbPage2 := groupPingKeyboard(groupID, users12, 2)
	// 2 user buttons + 1 pagination row + 1 add + 1 back = 5 rows
	if len(kbPage2.InlineKeyboard) != 5 {
		t.Fatalf("expected 5 rows on page 2, got %d", len(kbPage2.InlineKeyboard))
	}
	pagRow2 := kbPage2.InlineKeyboard[2]
	if pagRow2[0].Text != "◀️" || !strings.Contains(pagRow2[0].CallbackData, "ping_page:"+groupID+":1") {
		t.Errorf("expected prev button to page 1, got %+v", pagRow2[0])
	}
	if pagRow2[1].Text != "⏹️" || pagRow2[1].CallbackData != groupCallbackPrefix+"ping_noop" {
		t.Errorf("expected disabled next button on page 2, got %+v", pagRow2[1])
	}
}

func TestGroupPingSetConfirmationAndKeyboard(t *testing.T) {
	callerName := "Іван Петренко"
	groupName := "ІП-21"
	users := []string{"alice", "bob"}
	pendingID := uuid.New().String()
	var callerID int64 = 444555

	confirmText := formatGroupPingSetConfirm(callerName, groupName, users)
	if !strings.Contains(confirmText, "Іван Петренко") || !strings.Contains(confirmText, "ІП-21") {
		t.Errorf("expected caller and group in confirm text, got:\n%s", confirmText)
	}
	if !strings.Contains(confirmText, "@alice @bob") {
		t.Errorf("expected usernames in quote block, got:\n%s", confirmText)
	}
	if !strings.Contains(confirmText, "2") {
		t.Errorf("expected count 2 in confirm text, got:\n%s", confirmText)
	}

	kb := groupPingSetKeyboard(callerID, pendingID)
	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected 1 row with 2 buttons, got %+v", kb.InlineKeyboard)
	}
	proceedBtn := kb.InlineKeyboard[0][0]
	cancelBtn := kb.InlineKeyboard[0][1]

	if !strings.Contains(proceedBtn.Text, "Продовжити") {
		t.Errorf("expected proceed button text, got %q", proceedBtn.Text)
	}
	expectedProceedData := "gping:confirm:444555:" + pendingID
	if proceedBtn.CallbackData != expectedProceedData {
		t.Errorf("expected %q, got %q", expectedProceedData, proceedBtn.CallbackData)
	}

	if !strings.Contains(cancelBtn.Text, "Скасувати") {
		t.Errorf("expected cancel button text, got %q", cancelBtn.Text)
	}
	expectedCancelData := "gping:cancel:444555:" + pendingID
	if cancelBtn.CallbackData != expectedCancelData {
		t.Errorf("expected %q, got %q", expectedCancelData, cancelBtn.CallbackData)
	}

	successText := formatGroupPingSetSuccess(callerName, groupName, 2)
	if !strings.Contains(successText, "Іван Петренко") || !strings.Contains(successText, "успішно оновлено") || !strings.Contains(successText, "2") {
		t.Errorf("unexpected success text:\n%s", successText)
	}
}

func TestGroupPingConfigInGroupSettingsKeyboard(t *testing.T) {
	gid := uuid.New()
	g := setupTestGroup(gid, "ІП-21")

	kb := groupConfigKeyboard(g, true)
	foundPingBtn := false
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData == groupCallbackPrefix+"ping:"+gid.String() {
				foundPingBtn = true
				if !strings.Contains(btn.Text, "/ping") {
					t.Errorf("expected '/ping' in button text, got %q", btn.Text)
				}
			}
		}
	}
	if !foundPingBtn {
		t.Errorf("expected to find /ping settings button in groupConfigKeyboard")
	}
}

func setupTestGroup(id uuid.UUID, name string) model.BotGroup {
	var chatID int64 = -100999888
	return model.BotGroup{
		ID:                   id,
		AcademicGroupID:      4402,
		AcademicGroupName:    name,
		TelegramChatID:       &chatID,
		TelegramChatTitle:    "Чат " + name,
		NotificationsEnabled: true,
	}
}

func TestGroupPingFlow(t *testing.T) {
	_, db, _ := setupTestBot(t)
	ctx := context.Background()

	var chatID int64 = -10011223344
	group, err := db.CreateBotGroup(ctx, 111, 4402, "ІП-21", "ФІОТ", &chatID, "Чат ІП-21")
	if err != nil {
		t.Fatalf("creating test bot group: %v", err)
	}

	// 1. Initially no users
	users, err := db.GetGroupPingUsers(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroupPingUsers: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}

	// 2. Add users via prompt flow parsing
	rawInput := "@alice @bob @charlie @some_bot"
	valid, bots, invalid := parseUsernames(rawInput)
	if len(valid) != 3 || len(bots) != 1 || len(invalid) != 0 {
		t.Fatalf("unexpected parse result: valid=%+v bots=%+v invalid=%+v", valid, bots, invalid)
	}

	added, err := db.AddGroupPingUsers(ctx, group.ID, valid)
	if err != nil {
		t.Fatalf("AddGroupPingUsers: %v", err)
	}
	if added != 3 {
		t.Errorf("expected 3 added, got %d", added)
	}

	// 3. Check users stored
	users, err = db.GetGroupPingUsers(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroupPingUsers: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d: %+v", len(users), users)
	}

	// 4. Overwrite via /ping set pending confirmation flow
	pendingID := uuid.New()
	var adminID int64 = 111
	newPingList := []string{"david", "eva"}

	if err := db.SaveGroupPingPending(ctx, pendingID, group.ID, chatID, adminID, newPingList); err != nil {
		t.Fatalf("SaveGroupPingPending: %v", err)
	}

	pGID, pChatID, pUserID, pList, err := db.GetGroupPingPending(ctx, pendingID)
	if err != nil {
		t.Fatalf("GetGroupPingPending: %v", err)
	}
	if pGID != group.ID || pChatID != chatID || pUserID != adminID || len(pList) != 2 {
		t.Fatalf("mismatched pending record: gid=%s chat=%d user=%d list=%+v", pGID, pChatID, pUserID, pList)
	}

	// Confirming replaces users
	if err := db.SetGroupPingUsers(ctx, pGID, pList); err != nil {
		t.Fatalf("SetGroupPingUsers: %v", err)
	}
	if err := db.DeleteGroupPingPending(ctx, pendingID); err != nil {
		t.Fatalf("DeleteGroupPingPending: %v", err)
	}

	users, err = db.GetGroupPingUsers(ctx, group.ID)
	if err != nil {
		t.Fatalf("GetGroupPingUsers after set: %v", err)
	}
	if len(users) != 2 || users[0] != "david" || users[1] != "eva" {
		t.Fatalf("expected [david, eva], got %+v", users)
	}
}

