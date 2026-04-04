package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ListenAddr      string
	Namespace       string
	RefreshInterval int
	JobHistoryLimit int
	AuthUsername    string
	AuthPassword    string
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:   getEnv("LISTEN_ADDR", ":3000"),
		Namespace:    os.Getenv("NAMESPACE"),
		AuthUsername: os.Getenv("AUTH_USERNAME"),
		AuthPassword: os.Getenv("AUTH_PASSWORD"),
	}

	refreshInterval, err := getEnvInt("REFRESH_INTERVAL", 5)
	if err != nil {
		return nil, fmt.Errorf("REFRESH_INTERVAL: %w", err)
	}
	cfg.RefreshInterval = refreshInterval

	jobHistoryLimit, err := getEnvInt("JOB_HISTORY_LIMIT", 5)
	if err != nil {
		return nil, fmt.Errorf("JOB_HISTORY_LIMIT: %w", err)
	}
	cfg.JobHistoryLimit = jobHistoryLimit

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.AuthUsername == "" {
		return fmt.Errorf("AUTH_USERNAME is required")
	}
	if c.AuthPassword == "" {
		return fmt.Errorf("AUTH_PASSWORD is required")
	}
	if c.RefreshInterval < 1 {
		return fmt.Errorf("REFRESH_INTERVAL must be >= 1, got %d", c.RefreshInterval)
	}
	if c.JobHistoryLimit < 1 {
		return fmt.Errorf("JOB_HISTORY_LIMIT must be >= 1, got %d", c.JobHistoryLimit)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", v, err)
	}
	return n, nil
}
