package config

import (
	"os"
	"strings"
)

type Config struct {
	Environment string
	HTTPAddr    string
	DatabaseDSN string
	CORSOrigins []string
	JWTSecret   string
}

func Load() Config {
	return Config{
		Environment: valueOr("APP_ENV", "development"),
		HTTPAddr:    valueOr("HTTP_ADDR", ":8080"),
		DatabaseDSN: valueOr("DATABASE_DSN", "file:./data/paint-canvas.db"),
		CORSOrigins: split(valueOr("CORS_ORIGINS", "http://localhost:5174,http://127.0.0.1:5174")),
		JWTSecret:   valueOr("JWT_SECRET", "development-only-change-this-secret-before-deploying"),
	}
}

func valueOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func split(value string) []string {
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
