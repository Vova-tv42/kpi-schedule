package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

func TestAlertsAndNotifications(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_alerts.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// 1. User notifications toggle
	user, err := db.UpsertUser(ctx, 12345, nil, nil)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if !user.NotificationsEnabled {
		t.Errorf("expected NotificationsEnabled to default to true, got false")
	}

	usersWithNotif, err := db.GetUsersWithNotifications(ctx)
	if err != nil || len(usersWithNotif) != 1 {
		t.Fatalf("expected 1 user with notifications, got %d (err: %v)", len(usersWithNotif), err)
	}

	if err := db.SetUserNotifications(ctx, 12345, false); err != nil {
		t.Fatalf("set user notifications: %v", err)
	}
	uUpdated, err := db.GetUserByTelegramID(ctx, 12345)
	if err != nil || uUpdated.NotificationsEnabled {
		t.Errorf("expected NotificationsEnabled to be false, got %v", uUpdated.NotificationsEnabled)
	}
	usersWithNotif, _ = db.GetUsersWithNotifications(ctx)
	if len(usersWithNotif) != 0 {
		t.Errorf("expected 0 users with notifications after disabling, got %d", len(usersWithNotif))
	}

	// 2. Bot Group notifications toggle
	chatID := int64(-100987654)
	group, err := db.CreateBotGroup(ctx, 12345, 4402, "ІП-21", "ФІОТ", &chatID, "Group Chat")
	if err != nil {
		t.Fatalf("create bot group: %v", err)
	}
	if !group.NotificationsEnabled {
		t.Errorf("expected group NotificationsEnabled to default to true, got false")
	}

	activeGroups, err := db.GetActiveBotGroupsWithNotifications(ctx)
	if err != nil || len(activeGroups) != 1 {
		t.Fatalf("expected 1 active group with notifications, got %d (err: %v)", len(activeGroups), err)
	}

	if err := db.SetBotGroupNotifications(ctx, group.ID, false); err != nil {
		t.Fatalf("set bot group notifications: %v", err)
	}
	gUpdated, err := db.GetBotGroupByID(ctx, group.ID)
	if err != nil || gUpdated.NotificationsEnabled {
		t.Errorf("expected group NotificationsEnabled to be false, got %v", gUpdated.NotificationsEnabled)
	}
	activeGroups, _ = db.GetActiveBotGroupsWithNotifications(ctx)
	if len(activeGroups) != 0 {
		t.Errorf("expected 0 active groups after disabling, got %d", len(activeGroups))
	}

	// 3. Sent lesson alerts deduplication
	dateStr := "2026-09-03"
	timeStr := "08:30:00"
	recipientID := uuid.New().String()

	sent, err := db.HasAlertBeenSent(ctx, "user", recipientID, dateStr, timeStr, model.AlertBefore10m)
	if err != nil {
		t.Fatalf("check alert: %v", err)
	}
	if sent {
		t.Errorf("expected alert not sent yet")
	}

	if err := db.RecordAlertSent(ctx, "user", recipientID, dateStr, timeStr, model.AlertBefore10m); err != nil {
		t.Fatalf("record alert sent: %v", err)
	}

	sent, err = db.HasAlertBeenSent(ctx, "user", recipientID, dateStr, timeStr, model.AlertBefore10m)
	if err != nil || !sent {
		t.Fatalf("expected alert to be marked as sent, got %v (err: %v)", sent, err)
	}

	// Alert at start should still be false
	sentAtStart, err := db.HasAlertBeenSent(ctx, "user", recipientID, dateStr, timeStr, model.AlertAtStart)
	if err != nil || sentAtStart {
		t.Errorf("expected at_start alert to not be marked as sent yet")
	}

	// Test cleanup
	if err := db.CleanOldAlerts(ctx, 1*time.Hour); err != nil {
		t.Fatalf("clean old alerts: %v", err)
	}
}
