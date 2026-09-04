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

func TestGroupAdminsCRUD(t *testing.T) {
	ctx := context.Background()
	db, _, creatorID := setupTestDB(t)

	g, err := db.CreateBotGroup(ctx, creatorID, 4402, "ІП-21", "ФІОТ", nil, "")
	if err != nil {
		t.Fatalf("CreateBotGroup: %v", err)
	}

	rel, err := db.GetGroupAdminRelation(ctx, g.ID, creatorID)
	if err != nil {
		t.Fatalf("GetGroupAdminRelation creator: %v", err)
	}
	if rel != "creator" {
		t.Errorf("expected relation 'creator', got: %q", rel)
	}

	admin2ID := int64(888111)
	rel, err = db.GetGroupAdminRelation(ctx, g.ID, admin2ID)
	if err != nil {
		t.Fatalf("GetGroupAdminRelation admin2 before invite: %v", err)
	}
	if rel != "" {
		t.Errorf("expected relation '', got: %q", rel)
	}

	// Add admin2 as invited
	if err := db.AddGroupAdmin(ctx, g.ID, admin2ID, "admin2_user", "Admin 2"); err != nil {
		t.Fatalf("AddGroupAdmin: %v", err)
	}

	rel, err = db.GetGroupAdminRelation(ctx, g.ID, admin2ID)
	if err != nil {
		t.Fatalf("GetGroupAdminRelation admin2 after invite: %v", err)
	}
	if rel != "invited" {
		t.Errorf("expected relation 'invited', got: %q", rel)
	}

	// Admin2 should not see group in GetBotGroupsForUser while invited
	groups, err := db.GetBotGroupsForUser(ctx, admin2ID)
	if err != nil {
		t.Fatalf("GetBotGroupsForUser: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups for invited admin, got: %d", len(groups))
	}

	// Accept invitation
	if err := db.AcceptGroupAdmin(ctx, g.ID, admin2ID); err != nil {
		t.Fatalf("AcceptGroupAdmin: %v", err)
	}

	rel, err = db.GetGroupAdminRelation(ctx, g.ID, admin2ID)
	if err != nil {
		t.Fatalf("GetGroupAdminRelation after accept: %v", err)
	}
	if rel != "accepted" {
		t.Errorf("expected relation 'accepted', got: %q", rel)
	}

	// Admin2 should now see group in GetBotGroupsForUser
	groups, err = db.GetBotGroupsForUser(ctx, admin2ID)
	if err != nil {
		t.Fatalf("GetBotGroupsForUser after accept: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != g.ID {
		t.Fatalf("expected 1 group for accepted admin, got: %d", len(groups))
	}

	// Remove admin2
	if err := db.RemoveGroupAdmin(ctx, g.ID, admin2ID); err != nil {
		t.Fatalf("RemoveGroupAdmin: %v", err)
	}
	rel, err = db.GetGroupAdminRelation(ctx, g.ID, admin2ID)
	if err != nil {
		t.Fatalf("GetGroupAdminRelation after remove: %v", err)
	}
	if rel != "" {
		t.Errorf("expected relation '', got: %q", rel)
	}
}

func TestDeleteOrTransferGroupOwnership(t *testing.T) {
	ctx := context.Background()
	db, _, creatorID := setupTestDB(t)

	g, err := db.CreateBotGroup(ctx, creatorID, 4402, "ІП-21", "ФІОТ", nil, "")
	if err != nil {
		t.Fatalf("CreateBotGroup: %v", err)
	}

	admin2ID := int64(999222)
	admin3ID := int64(999333)

	_ = db.AddGroupAdmin(ctx, g.ID, admin2ID, "adm2", "Adm 2")
	_ = db.AddGroupAdmin(ctx, g.ID, admin3ID, "adm3", "Adm 3")
	_ = db.AcceptGroupAdmin(ctx, g.ID, admin2ID)
	// admin3 remains 'invited'

	// If admin2 leaves (non-creator):
	transferred, newOwner, err := db.DeleteOrTransferGroupOwnership(ctx, g.ID, admin2ID)
	if err != nil {
		t.Fatalf("co-admin leave: %v", err)
	}
	if transferred || newOwner != 0 {
		t.Errorf("co-admin leave should not transfer ownership")
	}
	rel, _ := db.GetGroupAdminRelation(ctx, g.ID, admin2ID)
	if rel != "" {
		t.Errorf("admin2 should be removed from group admins")
	}

	// Re-add admin2 and accept
	_ = db.AddGroupAdmin(ctx, g.ID, admin2ID, "adm2", "Adm 2")
	_ = db.AcceptGroupAdmin(ctx, g.ID, admin2ID)

	// Creator deletes group -> should transfer to admin2 (the only accepted admin)
	transferred, newOwner, err = db.DeleteOrTransferGroupOwnership(ctx, g.ID, creatorID)
	if err != nil {
		t.Fatalf("creator transfer: %v", err)
	}
	if !transferred || newOwner != admin2ID {
		t.Fatalf("expected transfer to admin2 (%d), got transferred=%v, newOwner=%d", admin2ID, transferred, newOwner)
	}

	updatedG, err := db.GetBotGroupByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetBotGroupByID after transfer: %v", err)
	}
	if updatedG.CreatorTelegramID != admin2ID {
		t.Errorf("expected creator to be admin2 (%d), got %d", admin2ID, updatedG.CreatorTelegramID)
	}

	// Now admin2 is creator and no other accepted admins exist.
	// Admin2 deletes group -> should purge completely.
	transferred, newOwner, err = db.DeleteOrTransferGroupOwnership(ctx, g.ID, admin2ID)
	if err != nil {
		t.Fatalf("final creator delete: %v", err)
	}
	if transferred {
		t.Errorf("expected no transfer when no accepted admins remain")
	}

	_, err = db.GetBotGroupByID(ctx, g.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for purged group, got: %v", err)
	}
}

