package config

import (
	"encoding/base64"
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL      string
	SessionKey       []byte // 32 bytes, decoded from SESSION_ENCRYPTION_KEY
	InternalAPIToken string
	HTTPAddr         string
	DebugRoutes      bool
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		InternalAPIToken: os.Getenv("INTERNAL_API_TOKEN"),
		HTTPAddr:         os.Getenv("HTTP_ADDR"),
		DebugRoutes:      os.Getenv("DEBUG_ROUTES") == "true",
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.InternalAPIToken == "" {
		return Config{}, fmt.Errorf("INTERNAL_API_TOKEN is required")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	rawKey := os.Getenv("SESSION_ENCRYPTION_KEY")
	if rawKey == "" {
		return Config{}, fmt.Errorf("SESSION_ENCRYPTION_KEY is required (base64, 32 bytes; generate with: openssl rand -base64 32)")
	}
	key, err := base64.StdEncoding.DecodeString(rawKey)
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_ENCRYPTION_KEY: invalid base64: %w", err)
	}
	if len(key) != 32 {
		return Config{}, fmt.Errorf("SESSION_ENCRYPTION_KEY: must decode to exactly 32 bytes, got %d", len(key))
	}
	cfg.SessionKey = key

	return cfg, nil
}
