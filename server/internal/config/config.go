// Package config reads the server's settings from environment variables.
package config

import "os"

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

// Load is called once from main. Nothing else in the server reads the
// environment, which keeps every other package testable with plain values.
func Load() Config {
	return Config{
		HTTPAddr: env("HTTP_ADDR", ":8080"),
		// No user or password: a Homebrew Postgres trusts local connections
		// and pgx defaults the user to the current OS user.
		DatabaseURL: env("DATABASE_URL", "postgres://localhost:5432/voice_translation?sslmode=disable"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
