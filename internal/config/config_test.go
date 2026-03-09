package config

import (
	"testing"
)

func TestLoad(t *testing.T) {
	// requiredEnv holds the minimum required env vars so Load() does not return an error.
	requiredEnv := map[string]string{
		"HTTP_BEARER_TOKEN":   "secret",
		"GOOGLE_REFRESH_TOKEN": "refresh",
	}

	setRequired := func(t *testing.T) {
		t.Helper()
		for k, v := range requiredEnv {
			t.Setenv(k, v)
		}
	}

	t.Run("defaults port to 8080", func(t *testing.T) {
		setRequired(t)
		t.Setenv("HTTP_PORT", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.HTTPPort != "8080" {
			t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "8080")
		}
	})

	t.Run("uses provided port", func(t *testing.T) {
		setRequired(t)
		t.Setenv("HTTP_PORT", "9090")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.HTTPPort != "9090" {
			t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "9090")
		}
	})

	t.Run("defaults timezone to America/New_York", func(t *testing.T) {
		setRequired(t)
		t.Setenv("DEFAULT_TIMEZONE", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.DefaultTimezone != "America/New_York" {
			t.Errorf("DefaultTimezone = %q, want %q", cfg.DefaultTimezone, "America/New_York")
		}
	})

	t.Run("error when HTTP_BEARER_TOKEN is missing", func(t *testing.T) {
		t.Setenv("HTTP_BEARER_TOKEN", "")
		t.Setenv("GOOGLE_REFRESH_TOKEN", "refresh")

		_, err := Load()
		if err == nil {
			t.Fatal("expected error for missing HTTP_BEARER_TOKEN, got nil")
		}
	})

	t.Run("error when GOOGLE_REFRESH_TOKEN is missing", func(t *testing.T) {
		t.Setenv("HTTP_BEARER_TOKEN", "secret")
		t.Setenv("GOOGLE_REFRESH_TOKEN", "")

		_, err := Load()
		if err == nil {
			t.Fatal("expected error for missing GOOGLE_REFRESH_TOKEN, got nil")
		}
	})
}
