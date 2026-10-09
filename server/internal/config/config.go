// Package config reads the server's settings from environment variables.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	AudioDir    string
	ModelsDir   string
	Workers     int

	// JobLease is how long a worker may go without a heartbeat before its
	// job is given to another. SweepInterval is how often that is checked.
	JobLease      time.Duration
	SweepInterval time.Duration

	// FakeFailAttempts makes the fake provider fail the first N attempts of
	// every job, to watch retries happen. 0 in normal use.
	FakeFailAttempts int
}

// Load is called once from main. Nothing else in the server reads the
// environment, which keeps every other package testable with plain values.
func Load() Config {
	return Config{
		HTTPAddr: env("HTTP_ADDR", ":8080"),
		// No user or password: a Homebrew Postgres trusts local connections
		// and pgx defaults the user to the current OS user.
		DatabaseURL: env("DATABASE_URL", "postgres://localhost:5432/voice_translation?sslmode=disable"),
		AudioDir:    env("AUDIO_DIR", "data/audio"),
		ModelsDir:   env("MODELS_DIR", "data/models"),
		Workers:     envInt("WORKERS", 4),

		JobLease:      envDuration("JOB_LEASE", 2*time.Minute),
		SweepInterval: envDuration("SWEEP_INTERVAL", 30*time.Second),

		FakeFailAttempts: envInt("FAKE_FAIL_ATTEMPTS", 0),
	}
}

func envInt(key string, fallback int) int {
	n, err := strconv.Atoi(os.Getenv(key))
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

// envDuration reads a value such as "30s" or "2m".
func envDuration(key string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(os.Getenv(key))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
