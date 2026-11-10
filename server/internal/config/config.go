// 文件说明：从环境变量和 .env 加载运行配置。

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv           string
	HTTPAddr         string
	DatabaseURL      string
	TestDatabaseURL  string
	JWTSecret        string
	JWTTTL           time.Duration
	PgDumpPath       string
	BackupDir        string
	QMSHTTPAddr      string
	QMSStaticDir     string
	QMSDeviceAliases string
	PortalURL        string
	IOTBaseURL       string
	IOTUsername      string
	IOTPassword      string
	IOTPressureID    string
	IOTPressureName  string
	IOTPressureLabel string
	IOTTHID          string
	IOTTHName        string
	IOTTHLabel       string
	IOTPowerStart    string
	IOTGapMinutes    int
	IOTQueryLimit    int
}

// Load 读取 .env 和环境变量，并校验数据库和 JWT 必需配置。
func Load() (*Config, error) {
	envFile := loadEnvFile()
	ttl := 12 * time.Hour
	if v := os.Getenv("JWT_TTL"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parse JWT_TTL: %w", err)
		}
		ttl = parsed
	}
	cfg := &Config{
		AppEnv:           envOr("APP_ENV", "development"),
		HTTPAddr:         envOr("HTTP_ADDR", ":8090"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		TestDatabaseURL:  os.Getenv("TEST_DATABASE_URL"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		JWTTTL:           ttl,
		PgDumpPath:       envOr("PG_DUMP_PATH", `D:\Program Files\PostgreSQL\18\bin\pg_dump.exe`),
		BackupDir:        envOr("BACKUP_DIR", `.\backups`),
		QMSHTTPAddr:      envOr("QMS_HTTP_ADDR", ":8081"),
		QMSStaticDir:     envOr("QMS_STATIC_DIR", `..\qms-web`),
		QMSDeviceAliases: envOr("QMS_DEVICE_ALIASES", "device-aliases.json"),
		PortalURL:        envOr("PORTAL_URL", ""),
		IOTBaseURL:       envOr("IOT_BASE_URL", "https://iot.cme-im.com"),
		IOTUsername:      os.Getenv("IOT_USERNAME"),
		IOTPassword:      os.Getenv("IOT_PASSWORD"),
		IOTPressureID:    envOr("IOT_PRESSURE_ID", "9bb10ca0-40e7-11f0-bd5b-695fce6e7bf5"),
		IOTPressureName:  envOr("IOT_PRESSURE_NAME", "CZDG-985-42"),
		IOTPressureLabel: envOr("IOT_PRESSURE_LABEL", "淬火线油烟治理系统"),
		IOTTHID:          envOr("IOT_TH_ID", "23f10b00-8c83-11f1-bd5b-695fce6e7bf5"),
		IOTTHName:        envOr("IOT_TH_NAME", "CZDG-TH"),
		IOTTHLabel:       envOr("IOT_TH_LABEL", "废气处理温湿度传感器"),
		IOTPowerStart:    envOr("IOT_POWER_HISTORY_START", "2025-07-09T00:00:00+08:00"),
		IOTGapMinutes:    envInt("IOT_GAP_MINUTES", 20),
		IOTQueryLimit:    envInt("IOT_QUERY_LIMIT", 200000),
	}
	baseDir := "."
	if envFile != "" {
		baseDir = filepath.Dir(envFile)
	}
	cfg.QMSStaticDir = resolvePath(baseDir, cfg.QMSStaticDir)
	cfg.QMSDeviceAliases = resolvePath(baseDir, cfg.QMSDeviceAliases)
	cfg.BackupDir = resolvePath(baseDir, cfg.BackupDir)
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

// loadEnvFile 允许从项目根目录或 server 目录启动，并返回实际读取的配置文件。
func loadEnvFile() string {
	candidates := []string{
		os.Getenv("ENV_FILE"),
		".env",
		filepath.Join("server", ".env"),
		filepath.Join("..", ".env"),
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil || seen[absolute] {
			continue
		}
		seen[absolute] = true
		info, err := os.Stat(absolute)
		if err != nil || info.IsDir() {
			continue
		}
		if godotenv.Load(absolute) == nil {
			return absolute
		}
	}
	return ""
}

func resolvePath(baseDir, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
		return fallback
	}
	return value
}
