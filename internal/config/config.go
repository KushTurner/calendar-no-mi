package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort    string
	BearerToken string
}

func Load() (*Config, error) {
	godotenv.Load()
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	return &Config{
		HTTPPort:    port,
		BearerToken: os.Getenv("BEARER_TOKEN"),
	}, nil
}
