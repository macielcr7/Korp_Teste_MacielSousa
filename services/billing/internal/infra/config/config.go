// Package config loads Billing runtime configuration.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config contains application settings loaded from the environment.
type Config struct {
	HTTPAddr               string
	DatabaseURL            string
	InventoryBaseURL       string
	InventoryInternalToken string
	InventoryHTTPTimeout   time.Duration
	WorkerPollInterval     time.Duration
	WorkerLeaseDuration    time.Duration
}

// Load reads and validates application settings.
func Load() (Config, error) {
	config := Config{
		HTTPAddr:               envOrDefault("HTTP_ADDR", ":8082"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		InventoryBaseURL:       envOrDefault("INVENTORY_BASE_URL", "http://inventory:8083"),
		InventoryInternalToken: strings.TrimSpace(os.Getenv("INVENTORY_INTERNAL_TOKEN")),
	}
	var err error
	if config.InventoryHTTPTimeout, err = duration("INVENTORY_HTTP_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if config.WorkerPollInterval, err = duration("WORKER_POLL_INTERVAL", time.Second); err != nil {
		return Config{}, err
	}
	if config.WorkerLeaseDuration, err = duration("WORKER_LEASE_DURATION", 30*time.Second); err != nil {
		return Config{}, err
	}
	if config.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if config.InventoryBaseURL == "" {
		return Config{}, fmt.Errorf("INVENTORY_BASE_URL is required")
	}
	if config.InventoryInternalToken == "" {
		return Config{}, fmt.Errorf("INVENTORY_INTERNAL_TOKEN is required")
	}
	if config.WorkerLeaseDuration <= config.InventoryHTTPTimeout {
		return Config{}, fmt.Errorf("WORKER_LEASE_DURATION must be greater than INVENTORY_HTTP_TIMEOUT")
	}
	return config, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	value := envOrDefault(key, fallback.String())
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return parsed, nil
}
