// Package config loads Inventory Service runtime configuration.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration.
type Config struct {
	HTTPAddr            string
	InternalHTTPAddr    string
	InternalAPIToken    string
	DatabaseURL         string
	DatabaseMaxOpen     int
	DatabaseMaxIdle     int
	DatabaseMaxIdleTime time.Duration
	DatabaseMaxLifetime time.Duration
}

// Load reads and validates application configuration from environment variables.
func Load() (Config, error) {
	config := Config{
		HTTPAddr:            environment("HTTP_ADDR", ":8081"),
		InternalHTTPAddr:    environment("INTERNAL_HTTP_ADDR", ":8083"),
		InternalAPIToken:    strings.TrimSpace(os.Getenv("INVENTORY_INTERNAL_TOKEN")),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		DatabaseMaxOpen:     environmentInteger("DATABASE_MAX_OPEN_CONNS", 20),
		DatabaseMaxIdle:     environmentInteger("DATABASE_MAX_IDLE_CONNS", 10),
		DatabaseMaxIdleTime: 5 * time.Minute,
		DatabaseMaxLifetime: 30 * time.Minute,
	}
	if config.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if config.InternalAPIToken == "" {
		return Config{}, errors.New("INVENTORY_INTERNAL_TOKEN is required")
	}
	return config, nil
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func environmentInteger(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
