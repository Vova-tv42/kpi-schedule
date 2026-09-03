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
