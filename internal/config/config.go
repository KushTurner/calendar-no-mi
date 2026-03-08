package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // no-op if .env absent
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	return &Config{
		HTTPPort: port,
	}, nil
}
