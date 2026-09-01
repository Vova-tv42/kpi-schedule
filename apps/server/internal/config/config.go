package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL      string
	InternalAPIToken string
	HTTPAddr         string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		InternalAPIToken: os.Getenv("INTERNAL_API_TOKEN"),
		HTTPAddr:         os.Getenv("HTTP_ADDR"),
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

	return cfg, nil
}
