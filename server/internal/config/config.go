// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config is shared by both the api and worker binaries.
type Config struct {
	DatabaseURL   string        // postgres connection string
	SessionSecret string        // used to derive/sign session tokens
	HTTPAddr      string        // api listen address, e.g. ":8080"
	WorkerID      string        // unique id for a worker instance
	ClaimBatch    int           // how many monitors a worker claims per tick
	LeaseDuration time.Duration // how long a claimed monitor stays leased
	PollInterval  time.Duration // how often a worker polls for due monitors
}

// Load reads configuration from the environment, applying sane defaults.
// DatabaseURL is the only strictly required value.
func Load() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionSecret: getenv("SESSION_SECRET", "dev-insecure-secret-change-me"),
		HTTPAddr:      getenv("HTTP_ADDR", ":8080"),
		WorkerID:      getenv("WORKER_ID", hostnameOr("worker")),
		ClaimBatch:    getenvInt("CLAIM_BATCH", 10),
		LeaseDuration: getenvDuration("LEASE_DURATION", 30*time.Second),
		PollInterval:  getenvDuration("POLL_INTERVAL", 2*time.Second),
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func hostnameOr(def string) string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return def
}
