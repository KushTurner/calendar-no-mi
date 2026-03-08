package config

import (
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantPort string
	}{
		{
			name:     "defaults port to 8080",
			env:      map[string]string{},
			wantPort: "8080",
		},
		{
			name:     "uses provided port",
			env:      map[string]string{"HTTP_PORT": "9090"},
			wantPort: "9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HTTP_PORT", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.HTTPPort != tt.wantPort {
				t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, tt.wantPort)
			}
		})
	}
}
