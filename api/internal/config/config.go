package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	RedisURL     string
	MasterKey    string
	CookieSecure bool
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:     env("HYPERVANE_HTTP_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		RedisURL:     env("REDIS_URL", "redis://localhost:6379/0"),
		MasterKey:    os.Getenv("HYPERVANE_MASTER_KEY"),
		CookieSecure: env("HYPERVANE_COOKIE_SECURE", "false") == "true",
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if strings.TrimSpace(cfg.MasterKey) == "" {
		return Config{}, fmt.Errorf("HYPERVANE_MASTER_KEY is required")
	}
	if len(cfg.MasterKey) < 16 {
		return Config{}, fmt.Errorf("HYPERVANE_MASTER_KEY must be at least 16 characters")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
