package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

func TestBotGroupsCRUD(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)

	// Create group without chat
	g, err := db.CreateBotGroup(ctx, telegramID, 4402, "ІП-21", "ФІОТ", nil, "")
	if err != nil {
		t.Fatalf("CreateBotGroup: %v", err)
	}
	if g.AcademicGroupName != "ІП-21" || g.Faculty != "ФІОТ" || g.TelegramChatID != nil {
		t.Fatalf("unexpected group: %+v", g)
	}

	// Get by ID
	fetched, err := db.GetBotGroupByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetBotGroupByID: %v", err)
	}
	if fetched.ID != g.ID || fetched.AcademicGroupName != "ІП-21" {
		t.Fatalf("mismatched fetched group: %+v", fetched)
	}

	// Get by creator
	creatorGroups, err := db.GetBotGroupsByCreator(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetBotGroupsByCreator: %v", err)
	}
	if len(creatorGroups) != 1 || creatorGroups[0].ID != g.ID {
		t.Fatalf("expected 1 group for creator, got: %d", len(creatorGroups))
	}

	// Bind chat
	chatID := int64(-100987654321)
	chatTitle := "ІП-21 Чат"
	if err := db.BindBotGroupChat(ctx, g.ID, chatID, chatTitle); err != nil {
		t.Fatalf("BindBotGroupChat: %v", err)
	}

	// Get by chat ID
	chatGroup, err := db.GetBotGroupByChatID(ctx, chatID)
	if err != nil {
		t.Fatalf("GetBotGroupByChatID: %v", err)
	}
	if chatGroup.TelegramChatID == nil || *chatGroup.TelegramChatID != chatID || chatGroup.TelegramChatTitle != chatTitle {
		t.Fatalf("unexpected chatGroup: %+v", chatGroup)
	}

	// Update academic
	if err := db.UpdateBotGroupAcademic(ctx, g.ID, 4403, "ІП-22", "ФІОТ"); err != nil {
		t.Fatalf("UpdateBotGroupAcademic: %v", err)
	}
	updated, err := db.GetBotGroupByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetBotGroupByID after update: %v", err)
	}
	if updated.AcademicGroupID != 4403 || updated.AcademicGroupName != "ІП-22" {
		t.Fatalf("academic info not updated: %+v", updated)
	}

	// Unbind chat
	if err := db.UnbindBotGroupChat(ctx, g.ID); err != nil {
		t.Fatalf("UnbindBotGroupChat: %v", err)
	}
	unbound, err := db.GetBotGroupByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetBotGroupByID after unbind: %v", err)
	}
	if unbound.TelegramChatID != nil || unbound.TelegramChatTitle != "" {
		t.Fatalf("expected nil chat id, got: %+v", unbound)
	}

	// Delete
	if err := db.DeleteBotGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteBotGroup: %v", err)
	}
	_, err = db.GetBotGroupByID(ctx, g.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after deletion, got: %v", err)
	}
}

func TestGroupPrompts(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)

	// No prompt initially
	prompt, err := db.GetGroupPrompt(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetGroupPrompt: %v", err)
	}
	if prompt != nil {
		t.Fatalf("expected nil prompt, got: %+v", prompt)
	}

	// Set prompt
	testGID := uuid.New()
	chatID := int64(-100111222333)
	err = db.SetGroupPrompt(ctx, model.GroupPrompt{
		TelegramID:      telegramID,
		PromptMessageID: 555,
		Action:          "edit_academic",
		GroupID:         &testGID,
		BindChatID:      &chatID,
		BindChatTitle:   "Some Chat",
	})
	if err != nil {
		t.Fatalf("SetGroupPrompt: %v", err)
	}

	// Get prompt
	prompt, err = db.GetGroupPrompt(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetGroupPrompt after set: %v", err)
	}
	if prompt == nil || prompt.Action != "edit_academic" || prompt.GroupID == nil || *prompt.GroupID != testGID {
		t.Fatalf("unexpected prompt: %+v", prompt)
	}
	if prompt.BindChatID == nil || *prompt.BindChatID != chatID || prompt.BindChatTitle != "Some Chat" {
		t.Fatalf("unexpected bind chat: %+v", prompt)
	}

	// Set prompt with URL action
	err = db.SetGroupPrompt(ctx, model.GroupPrompt{
		TelegramID:      telegramID,
		PromptMessageID: 556,
		Action:          "set_url",
		GroupID:         &testGID,
		SubjectNorm:     "math",
		Tag:             "lec",
		SubjectName:     "Higher Math",
	})
	if err != nil {
		t.Fatalf("SetGroupPrompt set_url: %v", err)
	}

	prompt, err = db.GetGroupPrompt(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetGroupPrompt after set_url: %v", err)
	}
	if prompt == nil || prompt.Action != "set_url" || prompt.SubjectNorm != "math" || prompt.Tag != "lec" || prompt.SubjectName != "Higher Math" {
		t.Fatalf("unexpected url prompt: %+v", prompt)
	}

	// Clear prompt
	if err := db.ClearGroupPrompt(ctx, telegramID); err != nil {
		t.Fatalf("ClearGroupPrompt: %v", err)
	}
	prompt, err = db.GetGroupPrompt(ctx, telegramID)
	if err != nil {
		t.Fatalf("GetGroupPrompt after clear: %v", err)
	}
	if prompt != nil {
		t.Fatalf("expected nil prompt after clear, got: %+v", prompt)
	}
}

func TestGroupLessonURLsCRUD(t *testing.T) {
	ctx := context.Background()
	db, _, telegramID := setupTestDB(t)

	// Create a bot group
	g, err := db.CreateBotGroup(ctx, telegramID, 4402, "ІП-21", "ФІОТ", nil, "")
	if err != nil {
		t.Fatalf("CreateBotGroup: %v", err)
	}

	// Initially empty
	urls, err := db.GetGroupLessonURLs(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGroupLessonURLs: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("expected 0 urls initially, got: %d", len(urls))
	}

	// Add a URL
	err = db.SetGroupLessonURL(ctx, g.ID, "programming", "lec", "https://zoom.us/j/123456")
	if err != nil {
		t.Fatalf("SetGroupLessonURL: %v", err)
	}

	// Add another URL
	err = db.SetGroupLessonURL(ctx, g.ID, "databases", "prac", "https://meet.google.com/abc-defg-hij")
	if err != nil {
		t.Fatalf("SetGroupLessonURL 2: %v", err)
	}

	urls, err = db.GetGroupLessonURLs(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGroupLessonURLs after set: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 urls, got: %d", len(urls))
	}
	if urls["programming|lec"] != "https://zoom.us/j/123456" {
		t.Errorf("unexpected url for programming|lec: %s", urls["programming|lec"])
	}
	if urls["databases|prac"] != "https://meet.google.com/abc-defg-hij" {
		t.Errorf("unexpected url for databases|prac: %s", urls["databases|prac"])
	}

	// Update URL
	err = db.SetGroupLessonURL(ctx, g.ID, "programming", "lec", "https://zoom.us/j/999999")
	if err != nil {
		t.Fatalf("SetGroupLessonURL update: %v", err)
	}
	urls, err = db.GetGroupLessonURLs(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGroupLessonURLs after update: %v", err)
	}
	if urls["programming|lec"] != "https://zoom.us/j/999999" {
		t.Errorf("expected updated url, got: %s", urls["programming|lec"])
	}

	// Delete URL
	err = db.DeleteGroupLessonURL(ctx, g.ID, "programming", "lec")
	if err != nil {
		t.Fatalf("DeleteGroupLessonURL: %v", err)
	}
	urls, err = db.GetGroupLessonURLs(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGroupLessonURLs after delete: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 url after delete, got: %d", len(urls))
	}
	if _, ok := urls["programming|lec"]; ok {
		t.Errorf("programming|lec should be deleted")
	}

	// Cascade delete when group is deleted
	if err := db.DeleteBotGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteBotGroup: %v", err)
	}
	urls, err = db.GetGroupLessonURLs(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetGroupLessonURLs after group delete: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("expected 0 urls after cascade delete, got: %d", len(urls))
	}
}
