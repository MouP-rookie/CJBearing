// 文件说明：Gin 服务器装配文件，定义中间件、路由分组和部门上下文。

package httpapi

import (
	"errors"
	"net/http"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/httpapi/middleware"
	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service"
	"bearing-inspection/internal/service/backup"
	"bearing-inspection/internal/service/instrument"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server 聚合配置、数据库、认证、授权、备份和仪表服务。
type Server struct {
	cfg         *config.Config
	db          *gorm.DB
	auth        *auth.Service
	authorizer  *rbac.Authorizer
	backups     *backup.Service
	instruments instrument.Ingestor
}

// NewServer 完成 HTTP 层依赖装配，不执行业务初始化。
func NewServer(cfg *config.Config, db *gorm.DB, authSvc *auth.Service, authorizer *rbac.Authorizer, backups *backup.Service, ingestor instrument.Ingestor) *Server {
	return &Server{cfg: cfg, db: db, auth: authSvc, authorizer: authorizer, backups: backups, instruments: ingestor}
}

// Router 定义公共、登录、PC 管理和执行接口的中间件边界。
func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.RequestID(), corsMiddleware())
	r.GET("/health", func(c *gin.Context) { response.OK(c, gin.H{"status": "ok", "time": time.Now()}) })

	api := r.Group("/api/v1")
	api.POST("/auth/pc/login", s.pcLogin)
	api.POST("/auth/pda/login", s.pdaLogin)
	api.GET("/ping", func(c *gin.Context) { response.OK(c, gin.H{"ok": true}) })
	api.GET("/settings/brand", s.getBrandSettings)

	secured := api.Group("")
	secured.Use(middleware.Auth(s.auth))
	secured.GET("/me", s.me)
	secured.POST("/auth/logout", s.logout)
	secured.GET("/settings/void-reasons", s.getVoidReasons)

	pc := secured.Group("/admin")
	pc.Use(middleware.RequireClient(auth.ClientPC))

	pc.PUT("/settings/brand", middleware.RequirePermission(s.authorizer, rbac.PermSystemManage), s.updateBrandSettings)
	pc.GET("/settings/brand", middleware.RequirePermission(s.authorizer, rbac.PermSystemManage), s.getScopedBrandSettings)
	pc.PUT("/settings/void-reasons", s.updateVoidReasons)
	pc.GET("/departments", middleware.RequirePermission(s.authorizer, rbac.PermDepartmentManage), s.listDepartments)
	pc.POST("/departments", middleware.RequirePermission(s.authorizer, rbac.PermDepartmentManage), s.createDepartment)
	pc.PATCH("/departments/:id", middleware.RequirePermission(s.authorizer, rbac.PermDepartmentManage), s.updateDepartment)
	pc.DELETE("/departments/:id", middleware.RequirePermission(s.authorizer, rbac.PermDepartmentManage), s.deleteDepartment)
	pc.GET("/users", middleware.RequirePermission(s.authorizer, rbac.PermUserManage), s.listUsers)
	pc.POST("/users", middleware.RequirePermission(s.authorizer, rbac.PermUserManage), s.createUser)
	pc.GET("/users/import-template", middleware.RequirePermission(s.authorizer, rbac.PermUserManage), s.userImportTemplate)
	pc.POST("/users/import", middleware.RequirePermission(s.authorizer, rbac.PermUserManage), s.importUsers)
	pc.PATCH("/users/:id", middleware.RequirePermission(s.authorizer, rbac.PermUserManage), s.updateUser)
	pc.POST("/users/:id/password", middleware.RequirePermission(s.authorizer, rbac.PermUserManage), s.resetUserPassword)
	pc.GET("/role-options", middleware.RequirePermission(s.authorizer, rbac.PermUserManage), s.listRoleOptions)
	pc.GET("/roles", middleware.RequirePermission(s.authorizer, rbac.PermRoleManage), s.listRoles)
	pc.POST("/roles", middleware.RequirePermission(s.authorizer, rbac.PermRoleManage), s.createRole)
	pc.PATCH("/roles/:id", middleware.RequirePermission(s.authorizer, rbac.PermRoleManage), s.updateRole)

	pc.GET("/lines", middleware.RequirePermission(s.authorizer, rbac.PermLineManage), s.listLines)
	pc.GET("/lines/import-template", middleware.RequirePermission(s.authorizer, rbac.PermLineManage), s.lineImportTemplate)
	pc.POST("/lines/import", middleware.RequirePermission(s.authorizer, rbac.PermLineManage), s.importLines)
	pc.POST("/lines", middleware.RequirePermission(s.authorizer, rbac.PermLineManage), s.createLine)
	pc.PATCH("/lines/:id", middleware.RequirePermission(s.authorizer, rbac.PermLineManage), s.updateLine)
	pc.DELETE("/lines/:id", middleware.RequirePermission(s.authorizer, rbac.PermLineManage), s.deleteLine)

	pc.GET("/product-line-options", middleware.RequirePermission(s.authorizer, rbac.PermProductManage), s.listProductLineOptions)
	pc.GET("/products", middleware.RequirePermission(s.authorizer, rbac.PermProductManage), s.listProducts)
	pc.POST("/products", middleware.RequirePermission(s.authorizer, rbac.PermProductManage), s.createProduct)
	pc.PATCH("/products/:id", middleware.RequirePermission(s.authorizer, rbac.PermProductManage), s.updateProduct)
	pc.DELETE("/products/:id", middleware.RequirePermission(s.authorizer, rbac.PermProductManage), s.deleteProduct)

	pc.GET("/shifts", middleware.RequirePermission(s.authorizer, rbac.PermShiftManage), s.listShiftPlans)
	pc.POST("/shifts", middleware.RequirePermission(s.authorizer, rbac.PermShiftManage), s.createShiftPlan)
	pc.PUT("/shifts/:id", middleware.RequirePermission(s.authorizer, rbac.PermShiftManage), s.updateShiftPlan)
	pc.DELETE("/shifts/:id", middleware.RequirePermission(s.authorizer, rbac.PermShiftManage), s.deleteShiftPlan)

	pc.GET("/form-templates", middleware.RequirePermission(s.authorizer, rbac.PermFormView), s.listFormTemplates)
	pc.POST("/form-templates", middleware.RequirePermission(s.authorizer, rbac.PermFormManage), s.createFormTemplate)
	pc.POST("/form-template-cleanup", middleware.RequirePermission(s.authorizer, rbac.PermFormManage), s.deleteUnusedFormTemplates)
	pc.POST("/form-template-delete", middleware.RequirePermission(s.authorizer, rbac.PermFormManage), s.deleteFormTemplates)
	pc.GET("/form-templates/:id", middleware.RequirePermission(s.authorizer, rbac.PermFormView), s.getFormTemplate)
	pc.POST("/form-templates/:id/versions", middleware.RequirePermission(s.authorizer, rbac.PermFormManage), s.createFormVersion)
	pc.POST("/form-versions/:id/publish", middleware.RequirePermission(s.authorizer, rbac.PermFormPublish), s.publishFormVersion)
	pc.POST("/form-versions/:id/rollback", middleware.RequirePermission(s.authorizer, rbac.PermFormRollback), s.rollbackFormVersion)
	pc.POST("/form-templates/:id/copy", middleware.RequirePermission(s.authorizer, rbac.PermFormManage), s.copyFormTemplate)
	pc.POST("/form-templates/:id/sync", middleware.RequirePermission(s.authorizer, rbac.PermFormManage), s.syncPairedTemplate)
	pc.GET("/form-templates/:id/assignments", middleware.RequirePermission(s.authorizer, rbac.PermFormView), s.listTemplateAssignments)
	pc.PUT("/form-templates/:id/assignments", middleware.RequirePermission(s.authorizer, rbac.PermFormManage), s.updateTemplateAssignments)
	pc.GET("/logs", middleware.RequirePermission(s.authorizer, rbac.PermLogView), s.listLogs)
	pc.GET("/logs/calendar", middleware.RequirePermission(s.authorizer, rbac.PermLogView), s.logCalendar)
	pc.POST("/submissions/:id/void", middleware.RequirePermission(s.authorizer, rbac.PermLogView), s.voidSubmission)
	pc.POST("/submissions/:id/restore", middleware.RequirePermission(s.authorizer, rbac.PermLogView), s.restoreSubmission)
	pc.GET("/audit-logs", middleware.RequirePermission(s.authorizer, rbac.PermAuditView), s.listAuditLogs)
	pc.POST("/exports", middleware.RequirePermission(s.authorizer, rbac.PermExport), s.exportData)
	pc.GET("/backups", middleware.RequirePermission(s.authorizer, rbac.PermBackupManage), s.listBackups)
	pc.POST("/backups", middleware.RequirePermission(s.authorizer, rbac.PermBackupManage), s.createBackup)
	pc.POST("/backups/restore", middleware.RequirePermission(s.authorizer, rbac.PermBackupManage), s.restoreBackup)
	pc.GET("/backups/:id/download", middleware.RequirePermission(s.authorizer, rbac.PermBackupManage), s.downloadBackup)
	pc.GET("/instrument-messages", middleware.RequirePermission(s.authorizer, rbac.PermLogView), s.listInstrumentMessages)

	exec := secured.Group("/execution")
	exec.GET("/lines", s.executionLines)
	exec.GET("/shifts", s.executionShifts)
	exec.GET("/repeat-check", s.repeatCheck)
	exec.GET("/products", middleware.RequirePermission(s.authorizer, rbac.PermExecutionView), s.executionProducts)
	exec.GET("/forms", middleware.RequirePermission(s.authorizer, rbac.PermExecutionView), s.executionForms)
	exec.GET("/forms/:id", middleware.RequirePermission(s.authorizer, rbac.PermExecutionView), s.executionFormDetail)
	exec.GET("/my-submissions", middleware.RequirePermission(s.authorizer, rbac.PermExecutionView), s.listMySubmissions)
	exec.GET("/my-submissions/:id", middleware.RequirePermission(s.authorizer, rbac.PermExecutionView), s.mySubmissionDetail)
	exec.GET("/submissions/current", middleware.RequirePermission(s.authorizer, rbac.PermExecutionView), s.currentSubmission)
	exec.GET("/submissions", middleware.RequirePermission(s.authorizer, rbac.PermExecutionView), s.listSubmissions)
	exec.POST("/submissions", middleware.RequirePermission(s.authorizer, rbac.PermExecutionFill), s.createSubmission)
	exec.PATCH("/rows/:id", s.updateSubmissionRow)
	exec.POST("/rows/:id/void", s.voidSubmissionRow)
	exec.POST("/rows/:id/restore", s.restoreSubmissionRow)

	r.NoRoute(func(c *gin.Context) { response.Fail(c, http.StatusNotFound, "not_found", "接口不存在", nil) })
	return r
}

// corsMiddleware 仅开发联调使用；生产环境应由反向代理限制来源。
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Department-ID")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// currentUser 重新读取数据库用户，确保 Token 签发后停用的账号立即失效。
func (s *Server) currentUser(c *gin.Context) (*model.User, error) {
	var user model.User
	err := s.db.Preload("Role").Preload("Department").First(&user, c.GetUint64("user_id")).Error
	if err != nil {
		return &user, err
	}
	if !user.Enabled || user.DeletedAt.Valid || user.Role.DeletedAt.Valid {
		return &user, errors.New("user disabled")
	}
	if user.Role.Code != "super_admin" && (user.DepartmentID == nil || user.Department == nil || !user.Department.Enabled || user.Department.DeletedAt.Valid) {
		return &user, errors.New("department disabled")
	}
	return &user, nil
}

// canLine 将功能权限与单条产线范围合并判断。
func (s *Server) canLine(userID uint64, permission string, lineID uint64) bool {
	ok, err := s.authorizer.Can(userID, permission, &lineID)
	return err == nil && ok
}

var _ = service.Seed
