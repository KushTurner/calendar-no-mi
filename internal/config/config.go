package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort              string
	OpenAIAPIKey          string
	HTTPBearerToken       string
	DefaultTimezone       string
	GoogleCredentialsFile string
	GoogleRefreshToken    string
	GoogleCalendarID      string
	// LogLevel controls slog verbosity (debug, info, warn, error). Invalid values fall back to info.
	LogLevel string
	// AppEnv controls log format: "production" → JSON to stdout, anything else → text to stderr.
	AppEnv string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // no-op if .env absent

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	timezone := os.Getenv("DEFAULT_TIMEZONE")
	if timezone == "" {
		timezone = "Europe/London"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}

	cfg := &Config{
		HTTPPort:              port,
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		HTTPBearerToken:       os.Getenv("HTTP_BEARER_TOKEN"),
		DefaultTimezone:       timezone,
		GoogleCredentialsFile: os.Getenv("GOOGLE_CREDENTIALS_FILE"),
		GoogleRefreshToken:    os.Getenv("GOOGLE_REFRESH_TOKEN"),
		GoogleCalendarID:      os.Getenv("GOOGLE_CALENDAR_ID"),
		LogLevel:              logLevel,
		AppEnv:                appEnv,
	}

	if cfg.HTTPBearerToken == "" {
		return nil, fmt.Errorf("config: HTTP_BEARER_TOKEN is required")
	}
	if cfg.GoogleRefreshToken == "" {
		return nil, fmt.Errorf("config: GOOGLE_REFRESH_TOKEN is required")
	}
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("config: OPENAI_API_KEY is required")
	}

	return cfg, nil
}
