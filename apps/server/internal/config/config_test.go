package config

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	t.Setenv("DATABASE_PATH", ":memory:")
	t.Setenv("INTERNAL_API_TOKEN", "test-token")

	// 1. Without bot token: should succeed
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected success without bot token, got: %v", err)
	}
	if cfg.TelegramBotToken != "" {
		t.Errorf("expected empty TelegramBotToken, got %q", cfg.TelegramBotToken)
	}

	// 2. With bot token but missing webhook URL: should fail
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:abc")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_WEBHOOK_URL is missing")
	}

	// 3. With bot token and webhook URL but missing secret: should fail
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:abc")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "https://example.com/api/v1/telegram/webhook")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_WEBHOOK_SECRET is missing")
	}

	// 4. With bot token, webhook URL, and secret: should succeed
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:abc")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "https://example.com/api/v1/telegram/webhook")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "super-secret")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("expected success with all webhook fields, got: %v", err)
	}
	if cfg.TelegramWebhookURL != "https://example.com/api/v1/telegram/webhook" {
		t.Errorf("unexpected TelegramWebhookURL: %s", cfg.TelegramWebhookURL)
	}
	if cfg.TelegramWebhookSecret != "super-secret" {
		t.Errorf("unexpected TelegramWebhookSecret: %s", cfg.TelegramWebhookSecret)
	}
}
