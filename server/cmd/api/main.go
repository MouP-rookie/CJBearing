package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/database"
	"bearing-inspection/internal/httpapi"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service/backup"
	"bearing-inspection/internal/service/instrument"
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
	authorizer := rbac.NewAuthorizer(db)
	backups := backup.New(db, cfg.PgDumpPath, cfg.BackupDir)
	ingestor := instrument.NewDBStore(db)
	server := httpapi.NewServer(cfg, db, authSvc, authorizer, backups, ingestor)
	router := server.Router()
	router.StaticFS("/pc", http.Dir("../web-pc/dist"))
	router.StaticFS("/pda", http.Dir("../web-pda/dist"))
	router.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/pc/") })
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: router, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		slog.Info("server started", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if errors.Is(err, syscall.EADDRINUSE) || strings.Contains(err.Error(), "Only one usage") {
				slog.Error("API启动失败：8080端口已被占用，请先停止已运行的API或cmd/dev进程")
			} else {
				slog.Error("server stopped", "error", err)
			}
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
