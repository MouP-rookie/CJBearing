package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv          string
	HTTPAddr        string
	DatabaseURL     string
	TestDatabaseURL string
	JWTSecret       string
	JWTTTL          time.Duration
	PgDumpPath      string
	BackupDir       string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	ttl := 12 * time.Hour
	if v := os.Getenv("JWT_TTL"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse JWT_TTL: %w", err)
		}
		ttl = parsed
	}
	cfg := &Config{
		AppEnv:          envOr("APP_ENV", "development"),
		HTTPAddr:        envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		TestDatabaseURL: os.Getenv("TEST_DATABASE_URL"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTTTL:          ttl,
		PgDumpPath:      envOr("PG_DUMP_PATH", `D:\Program Files\PostgreSQL\18\bin\pg_dump.exe`),
		BackupDir:       envOr("BACKUP_DIR", `.\backups`),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
