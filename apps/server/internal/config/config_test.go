package config

import (
	"testing"
	"time"
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

func TestConfigIdleTimeout(t *testing.T) {
	t.Setenv("DATABASE_PATH", ":memory:")
	t.Setenv("INTERNAL_API_TOKEN", "test-token")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")

	// 1. Unset: defaults to 0
	t.Setenv("IDLE_TIMEOUT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected success with unset IDLE_TIMEOUT, got: %v", err)
	}
	if cfg.IdleTimeout != 0 {
		t.Errorf("expected 0 for unset IDLE_TIMEOUT, got %v", cfg.IdleTimeout)
	}

	// 2. Valid duration: 15m
	t.Setenv("IDLE_TIMEOUT", "15m")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("expected success with valid IDLE_TIMEOUT, got: %v", err)
	}
	if cfg.IdleTimeout != 15*time.Minute {
		t.Errorf("expected 15m for IDLE_TIMEOUT, got %v", cfg.IdleTimeout)
	}

	// 3. Invalid format: returns error
	t.Setenv("IDLE_TIMEOUT", "invalid-duration")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error for invalid IDLE_TIMEOUT")
	}

	// 4. Negative duration: returns error
	t.Setenv("IDLE_TIMEOUT", "-5m")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error for negative IDLE_TIMEOUT")
	}
}
