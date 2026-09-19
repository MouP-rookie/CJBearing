// 文件说明：创建 GORM PostgreSQL 连接并配置日志级别。

package database

import (
	"log/slog"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 建立 PostgreSQL 连接；只输出 GORM 警告及以上日志，避免业务日志过噪。
func Open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

type LogWriter struct{}

func (LogWriter) Printf(format string, args ...any) {
	slog.Debug(strings.TrimSpace(format), args...)
}
