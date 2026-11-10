// 文件说明：Go 版 QMS API 与静态前端服务，沿用现有前端请求协议。

package qms

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	cfg        *config.Config
	db         *gorm.DB
	auth       *auth.Service
	authorizer *rbac.Authorizer
	catalog    *DeviceCatalog
	history    *HistoryService
	overview   *TelemetryService
}

func NewServer(cfg *config.Config, db *gorm.DB, authSvc *auth.Service, authorizer *rbac.Authorizer) *Server {
	client := NewThingsBoardClient(cfg.IOTBaseURL, cfg.IOTUsername, cfg.IOTPassword)
	catalog := NewDeviceCatalog(client)
	aliases, err := LoadDeviceAliases(cfg.QMSDeviceAliases)
	if err != nil {
		slog.Warn("加载设备改号映射失败，历史查询将不合并改号前数据", "path", cfg.QMSDeviceAliases, "error", err)
		aliases = nil
	} else if len(aliases) > 0 {
		slog.Info("设备改号映射已加载", "path", cfg.QMSDeviceAliases, "devices", len(aliases))
	}
	return &Server{
		cfg: cfg, db: db, auth: authSvc, authorizer: authorizer,
		catalog: catalog, history: NewHistoryService(client, catalog, aliases),
		overview: NewTelemetryService(client, cfg),
	}
}

func (s *Server) Router() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now()})
	})

	api := router.Group("/api")
	api.Use(s.requireSystemAccess)
	api.GET("/overview", s.requirePermission(rbac.PermQMSView), func(c *gin.Context) {
		result, err := Overview(s.cfg, s.overview, ParseQuery(c.Request.URL.Query()))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	})
	api.GET("/devices", s.requirePermission(rbac.PermQMSHistory), func(c *gin.Context) {
		devices, err := s.catalog.Devices(strings.EqualFold(c.Query("refresh"), "true"))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"count": len(devices), "devices": devices})
	})
	api.GET("/history", s.requirePermission(rbac.PermQMSHistory), s.historyHandler(false))
	api.GET("/history/page", s.requirePermission(rbac.PermQMSHistory), s.historyHandler(false))
	api.GET("/history/raw", s.requirePermission(rbac.PermQMSHistory), s.historyHandler(true))
	api.GET("/history/export", s.requirePermission(rbac.PermQMSExport), s.exportHandler)

	staticMiddleware := []gin.HandlerFunc{s.requireSystemAccess, s.requirePermission(rbac.PermQMSView)}
	router.GET("/", append(staticMiddleware, func(c *gin.Context) {
		c.File(filepath.Join(s.cfg.QMSStaticDir, "index.html"))
	})...)
	staticFiles := map[string]string{
		"/style.css":                 "style.css",
		"/app.js":                    "app.js",
		"/vendor/vue.global.prod.js": filepath.Join("vendor", "vue.global.prod.js"),
	}
	for route, file := range staticFiles {
		route, file := route, file
		router.GET(route, append(staticMiddleware, func(c *gin.Context) {
			c.File(filepath.Join(s.cfg.QMSStaticDir, file))
		})...)
	}

	router.NoRoute(s.requireSystemAccess, s.requirePermission(rbac.PermQMSView), func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		index := filepath.Join(s.cfg.QMSStaticDir, "index.html")
		if _, err := os.Stat(index); err != nil {
			c.String(http.StatusNotFound, "QMS frontend not found")
			return
		}
		c.File(index)
	})
	return router
}

func (s *Server) historyHandler(raw bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := strings.TrimSpace(c.Query("deviceId"))
		if deviceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请选择设备"})
			return
		}
		from := ParseTime(c.Query("start"))
		to := ParseTime(c.Query("end"))
		limit := queryInt(c, "limit", 0)
		order := c.DefaultQuery("order", "DESC")
		if raw {
			payload, err := s.history.Raw(deviceID, from, to, limit, order)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
			c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
			return
		}
		var result map[string]any
		var err error
		if strings.HasSuffix(c.Request.URL.Path, "/page") {
			if before := c.Query("before"); before != "" {
				value, parseErr := strconv.ParseInt(before, 10, 64)
				if parseErr != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "无效分页游标"})
					return
				}
				if to == nil || value < *to {
					to = &value
				}
			}
			result, err = s.history.BuildPage(deviceID, from, to)
		} else {
			result, err = s.history.Build(deviceID, from, to, limit, order)
		}
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func (s *Server) exportHandler(c *gin.Context) {
	deviceID := strings.TrimSpace(c.Query("deviceId"))
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择设备"})
		return
	}
	mode := c.DefaultQuery("mode", "group")
	filename, content, err := s.history.Export(
		deviceID, ParseTime(c.Query("start")), ParseTime(c.Query("end")),
		queryInt(c, "limit", 0), c.DefaultQuery("order", "DESC"),
		mode, c.Query("keys"), strings.EqualFold(c.Query("fill"), "true") || c.Query("fill") == "1",
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	contentType := "text/csv; charset=utf-8"
	if strings.EqualFold(mode, "raw") {
		contentType = "application/json; charset=utf-8"
	}
	encoded := url.QueryEscape(filename)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+encoded)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, contentType, []byte(content))
}

func (s *Server) requireSystemAccess(c *gin.Context) {
	raw := ""
	if header := c.GetHeader("Authorization"); strings.HasPrefix(strings.ToLower(header), "bearer ") {
		raw = strings.TrimSpace(header[7:])
	}
	if raw == "" {
		if cookie, err := c.Cookie("token"); err == nil {
			raw = cookie
		}
	}
	if raw == "" {
		s.deny(c, false)
		return
	}
	claims, err := s.auth.ParseToken(raw)
	if err != nil {
		s.deny(c, false)
		return
	}
	if claims.Client != auth.ClientPC && claims.Client != auth.ClientPortal {
		s.deny(c, false)
		return
	}
	var user model.User
	if err := s.db.Preload("Role").Preload("Department").First(&user, claims.UserID).Error; err != nil {
		s.deny(c, false)
		return
	}
	if !user.Enabled || user.DeletedAt.Valid || user.Role.DeletedAt.Valid {
		s.deny(c, false)
		return
	}
	if user.Role.Code != "super_admin" {
		if user.DepartmentID == nil || user.Department == nil || !user.Department.Enabled || user.Department.DeletedAt.Valid {
			s.deny(c, true)
			return
		}
		var count int64
		err := s.db.Table("portal_system_access").
			Where("user_id = ? AND system_code = ? AND enabled = ?", user.ID, "qms_iot", true).
			Count(&count).Error
		if err != nil || count == 0 {
			s.deny(c, true)
			return
		}
	}
	c.Set("qms_user_id", user.ID)
	c.Set("qms_user", &user)
	c.Next()
}

func (s *Server) requirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("qms_user_id")
		allowed, err := s.authorizer.Can(userID, permission, nil)
		if err != nil || !allowed {
			s.deny(c, true)
			return
		}
		c.Next()
	}
}

func (s *Server) deny(c *gin.Context, authenticated bool) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		code := http.StatusUnauthorized
		message := "请先通过门户登录"
		if authenticated {
			code = http.StatusForbidden
			message = "没有 QMS 质量管理系统权限"
		}
		c.AbortWithStatusJSON(code, gin.H{"error": message})
		return
	}
	if authenticated {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Redirect(http.StatusFound, s.portalRedirect(c))
	c.Abort()
}

func (s *Server) portalRedirect(c *gin.Context) string {
	if s.cfg.PortalURL != "" {
		return s.cfg.PortalURL
	}
	host := c.Request.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	protocol := c.GetHeader("X-Forwarded-Proto")
	if protocol == "" {
		protocol = "http"
		if c.Request.TLS != nil {
			protocol = "https"
		}
	}
	return protocol + "://" + net.JoinHostPort(host, "8090") + "/portal/"
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return value
}

func writeJSON(c *gin.Context, status int, payload any) {
	c.Status(status)
	c.Header("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(c.Writer).Encode(payload)
}

var _ = fmt.Sprint
