package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabasePath     string
	InternalAPIToken string
	HTTPAddr         string
	IdleTimeout      time.Duration
	// TelegramBotToken is optional: if empty, the bot is not started and the
	// server runs API-only, as it did before the bot existed.
	TelegramBotToken      string
	TelegramWebhookURL    string
	TelegramWebhookSecret string
	ExtensionInstallURL   string
	CronSecret            string
	AdminAPISecret        string
	AdminIngestURL        string
	AdminIngestKey        string
}

func Load() (Config, error) {
	// Best-effort: only present in local dev. In Docker/production, real env
	// vars are already set and no .env file exists.
	_ = godotenv.Load()

	cfg := Config{
		DatabasePath:          os.Getenv("DATABASE_PATH"),
		InternalAPIToken:      os.Getenv("INTERNAL_API_TOKEN"),
		HTTPAddr:              os.Getenv("HTTP_ADDR"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramWebhookURL:    os.Getenv("TELEGRAM_WEBHOOK_URL"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		ExtensionInstallURL:   os.Getenv("EXTENSION_INSTALL_URL"),
		CronSecret:            os.Getenv("CRON_SECRET"),
		AdminAPISecret:        os.Getenv("ADMIN_API_SECRET"),
		AdminIngestURL:        os.Getenv("ADMIN_INGEST_URL"),
		AdminIngestKey:        os.Getenv("ADMIN_INGEST_KEY"),
	}
	if cfg.ExtensionInstallURL == "" {
		cfg.ExtensionInstallURL = os.Getenv("EXTENSION_DOWNLOAD_URL")
	}
	if cfg.CronSecret == "" {
		cfg.CronSecret = cfg.InternalAPIToken
	}
	if cfg.AdminAPISecret == "" {
		cfg.AdminAPISecret = cfg.InternalAPIToken
	}

	if cfg.DatabasePath == "" {
		return Config{}, fmt.Errorf("DATABASE_PATH is required")
	}
	if cfg.InternalAPIToken == "" {
		return Config{}, fmt.Errorf("INTERNAL_API_TOKEN is required")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if rawIdle := os.Getenv("IDLE_TIMEOUT"); rawIdle != "" {
		d, err := time.ParseDuration(rawIdle)
		if err != nil {
			return Config{}, fmt.Errorf("invalid IDLE_TIMEOUT %q: %w", rawIdle, err)
		}
		if d < 0 {
			return Config{}, fmt.Errorf("IDLE_TIMEOUT cannot be negative: %v", d)
		}
		cfg.IdleTimeout = d
	}

	if cfg.TelegramBotToken != "" {
		if cfg.TelegramWebhookURL == "" {
			return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_URL is required when TELEGRAM_BOT_TOKEN is set")
		}
		if cfg.TelegramWebhookSecret == "" {
			return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET is required when TELEGRAM_BOT_TOKEN is set")
		}
	}

	return cfg, nil
}
