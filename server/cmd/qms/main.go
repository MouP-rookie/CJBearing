// 文件说明：Go 版 QMS 质量管理和静态前端启动入口。

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/database"
	"bearing-inspection/internal/qms"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTTTL)
	server := qms.NewServer(cfg, db, authSvc, rbac.NewAuthorizer(db)).Router()
	httpServer := &http.Server{
		Addr: cfg.QMSHTTPAddr, Handler: server, ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("QMS Go server started", "addr", cfg.QMSHTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("QMS server stopped", "error", err)
			os.Exit(1)
		}
	}()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

var _ = gin.ReleaseMode
