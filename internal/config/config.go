package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort    string
	BearerToken string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // no-op if .env absent
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	token := os.Getenv("BEARER_TOKEN")
	if token == "" {
		return nil, errors.New("BEARER_TOKEN is required but not set")
	}
	return &Config{HTTPPort: port, BearerToken: token}, nil
}
