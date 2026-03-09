package config

import (
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
}

func Load() (*Config, error) {
	_ = godotenv.Load() // no-op if .env absent

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	timezone := os.Getenv("DEFAULT_TIMEZONE")
	if timezone == "" {
		timezone = "America/New_York"
	}

	return &Config{
		HTTPPort:              port,
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		HTTPBearerToken:       os.Getenv("HTTP_BEARER_TOKEN"),
		DefaultTimezone:       timezone,
		GoogleCredentialsFile: os.Getenv("GOOGLE_CREDENTIALS_FILE"),
		GoogleRefreshToken:    os.Getenv("GOOGLE_REFRESH_TOKEN"),
		GoogleCalendarID:      os.Getenv("GOOGLE_CALENDAR_ID"),
	}, nil
}
