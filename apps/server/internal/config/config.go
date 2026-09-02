package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabasePath     string
	InternalAPIToken string
	HTTPAddr         string
}

func Load() (Config, error) {
	// Best-effort: only present in local dev. In Docker/production, real env
	// vars are already set and no .env file exists.
	_ = godotenv.Load()

	cfg := Config{
		DatabasePath:     os.Getenv("DATABASE_PATH"),
		InternalAPIToken: os.Getenv("INTERNAL_API_TOKEN"),
		HTTPAddr:         os.Getenv("HTTP_ADDR"),
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

	return cfg, nil
}
