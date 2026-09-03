package bot_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/bot"
	"kpi-schedule-bot/server/internal/storage"
)

func TestUserSettings(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_bot_settings.db")
	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	b, err := bot.New("123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", nil, db, &gotgbot.BotOpts{
		DisableTokenCheck: true,
	})
	if err != nil {
		t.Fatalf("creating bot: %v", err)
	}

	if b.GBot() == nil {
		t.Errorf("expected GBot() to not be nil")
	}

	user, err := db.UpsertUser(ctx, 555444, nil, nil)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	if !user.NotificationsEnabled {
		t.Errorf("expected NotificationsEnabled to be true by default")
	}

	if err := db.SetUserNotifications(ctx, 555444, false); err != nil {
		t.Fatalf("set user notifications: %v", err)
	}
	updated, _ := db.GetUserByTelegramID(ctx, 555444)
	if updated.NotificationsEnabled {
		t.Errorf("expected NotificationsEnabled to be false after toggle")
	}
}
