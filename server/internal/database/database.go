package database

import (
	"log/slog"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

type LogWriter struct{}

func (LogWriter) Printf(format string, args ...any) {
	slog.Debug(strings.TrimSpace(format), args...)
}
