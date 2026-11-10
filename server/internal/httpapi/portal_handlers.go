// 文件说明：门户系统注册、跨系统查看授权和子系统权限目录接口。

package httpapi

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type portalSystem struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	EntryPath   string    `json:"entryPath"`
	EntryPort   int       `json:"entryPort"`
	HealthPath  string    `json:"healthPath"`
	Accent      string    `json:"accent"`
	Icon        string    `json:"icon"`
	Enabled     bool      `json:"enabled"`
	SortOrder   int       `json:"sortOrder"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Allowed     bool      `json:"allowed"`
}

type portalUserSummary struct {
	ID            uint64   `json:"id"`
	Account       string   `json:"account"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	RoleID        uint64   `json:"roleId"`
	RoleCode      string   `json:"roleCode"`
	RoleName      string   `json:"roleName"`
	DepartmentID  *uint64  `json:"departmentId,omitempty"`
	Department    string   `json:"department"`
	SystemCodes   []string `json:"systemCodes"`
	IsSuperAdmin  bool     `json:"isSuperAdmin"`
	PermissionCnt int      `json:"permissionCount"`
}

// portalSystems returns every enabled system plus whether the current user may enter it.
func (s *Server) portalSystems(c *gin.Context) {
	user, err := s.currentUser(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "用户不存在或已停用", nil)
		return
	}
	var rows []portalSystem
	if err := s.db.Table("portal_systems").
		Select("code, name, description, entry_path, entry_port, health_path, accent, icon, enabled, sort_order, updated_at").
		Where("enabled = ?", true).
		Order("sort_order, code").Scan(&rows).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "portal_systems_failed", "读取系统目录失败", err.Error())
		return
	}
	allowed := map[string]bool{}
	if user.Role.Code == "super_admin" {
		for i := range rows {
			rows[i].Allowed = true
		}
	} else {
		var codes []string
		_ = s.db.Table("portal_system_access").
			Where("user_id = ? AND enabled = ?", user.ID, true).
			Pluck("system_code", &codes).Error
		for _, code := range codes {
			allowed[code] = true
		}
		for i := range rows {
			rows[i].Allowed = allowed[rows[i].Code]
		}
	}
	response.OK(c, gin.H{
		"systems":      rows,
		"user":         publicUser(user),
		"isSuperAdmin": user.Role.Code == "super_admin",
	})
}

func (s *Server) portalAdminSystems(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	var rows []portalSystem
	if err := s.db.Table("portal_systems").
		Select("code, name, description, entry_path, entry_port, health_path, accent, icon, enabled, sort_order, updated_at").
		Order("sort_order, code").Scan(&rows).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "portal_systems_failed", "读取系统目录失败", err.Error())
		return
	}
	response.OK(c, gin.H{"systems": rows})
}

type portalSystemUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	EntryPath   *string `json:"entryPath"`
	EntryPort   *int    `json:"entryPort"`
	HealthPath  *string `json:"healthPath"`
	Accent      *string `json:"accent"`
	Icon        *string `json:"icon"`
	Enabled     *bool   `json:"enabled"`
	SortOrder   *int    `json:"sortOrder"`
}

func (s *Server) portalAdminUpdateSystem(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	var req portalSystemUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_request", "系统配置格式不正确", err.Error())
		return
	}
	updates := map[string]any{"updated_at": time.Now()}
	setString := func(key string, value *string, trim bool) {
		if value == nil {
			return
		}
		v := *value
		if trim {
			v = strings.TrimSpace(v)
		}
		updates[key] = v
	}
	setString("name", req.Name, true)
	setString("description", req.Description, true)
	setString("entry_path", req.EntryPath, true)
	setString("health_path", req.HealthPath, true)
	setString("accent", req.Accent, true)
	setString("icon", req.Icon, true)
	if req.EntryPort != nil {
		if *req.EntryPort < 1 || *req.EntryPort > 65535 {
			response.Fail(c, http.StatusBadRequest, "invalid_port", "系统端口必须在 1 到 65535 之间", nil)
			return
		}
		updates["entry_port"] = *req.EntryPort
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	result := s.db.Table("portal_systems").Where("code = ?", c.Param("code")).Updates(updates)
	if result.Error != nil {
		response.Fail(c, http.StatusInternalServerError, "system_update_failed", "保存系统配置失败", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		response.Fail(c, http.StatusNotFound, "system_not_found", "系统不存在", nil)
		return
	}
	response.NoContent(c)
}

func (s *Server) portalAdminUsers(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	var users []model.User
	if err := s.db.Preload("Role").Preload("Department").
		Where("deleted_at IS NULL").Order("enabled DESC, account").Find(&users).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "portal_users_failed", "读取用户失败", err.Error())
		return
	}
	accessRows := []struct {
		UserID     uint64 `gorm:"column:user_id"`
		SystemCode string `gorm:"column:system_code"`
	}{}
	_ = s.db.Table("portal_system_access").Where("enabled = ?", true).Find(&accessRows).Error
	byUser := map[uint64][]string{}
	for _, row := range accessRows {
		byUser[row.UserID] = append(byUser[row.UserID], row.SystemCode)
	}
	items := make([]portalUserSummary, 0, len(users))
	for _, user := range users {
		codes := append([]string{}, byUser[user.ID]...)
		if codes == nil {
			codes = []string{}
		}
		sort.Strings(codes)
		departmentName := "全局"
		if user.Department != nil {
			departmentName = user.Department.Name
		}
		items = append(items, portalUserSummary{
			ID: user.ID, Account: user.Account, Name: user.Name, Enabled: user.Enabled,
			RoleID: user.RoleID, RoleCode: user.Role.Code, RoleName: user.Role.Name,
			DepartmentID: user.DepartmentID, Department: departmentName,
			SystemCodes: codes, IsSuperAdmin: user.Role.Code == "super_admin",
		})
	}
	response.OK(c, gin.H{"items": items})
}

type accessUpdateRequest struct {
	SystemCodes []string `json:"systemCodes"`
}

func (s *Server) portalAdminUpdateAccess(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	userID, ok := parsePathUint(c, "id")
	if !ok {
		response.Fail(c, http.StatusBadRequest, "invalid_user", "用户 ID 无效", nil)
		return
	}
	var target model.User
	if err := s.db.Preload("Role").First(&target, userID).Error; err != nil {
		response.Fail(c, http.StatusNotFound, "user_not_found", "用户不存在", nil)
		return
	}
	if target.Role.Code == "super_admin" {
		response.Fail(c, http.StatusForbidden, "super_admin_locked", "超级管理员默认拥有所有系统，不能单独修改", nil)
		return
	}
	var req accessUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_request", "权限参数格式不正确", err.Error())
		return
	}
	codes := make([]string, 0, len(req.SystemCodes))
	seen := map[string]bool{}
	for _, raw := range req.SystemCodes {
		code := strings.TrimSpace(raw)
		if code != "" && !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	if len(codes) > 0 {
		var count int64
		if err := s.db.Table("portal_systems").Where("code IN ? AND enabled = ?", codes, true).Count(&count).Error; err != nil || count != int64(len(codes)) {
			response.Fail(c, http.StatusBadRequest, "invalid_system", "包含不存在或已停用的系统", nil)
			return
		}
	}
	actorID := c.GetUint64("user_id")
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("portal_system_access").Where("user_id = ?", userID).Delete(nil).Error; err != nil {
			return err
		}
		for _, code := range codes {
			if err := tx.Exec(`
				INSERT INTO portal_system_access (user_id, system_code, enabled, updated_by, updated_at)
				VALUES (?, ?, TRUE, ?, NOW())`, userID, code, actorID).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "access_update_failed", "保存系统权限失败", err.Error())
		return
	}
	s.audit(c, "portal_system_access", userID, "update", nil, gin.H{"systemCodes": codes})
	response.NoContent(c)
}

type portalRoleSummary struct {
	ID              uint64   `json:"id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	IsSystem        bool     `json:"isSystem"`
	AllowPC         bool     `json:"allowPc"`
	AllowPDA        bool     `json:"allowPda"`
	PermissionCodes []string `json:"permissionCodes"`
}

func (s *Server) portalAdminRoles(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	var roles []model.Role
	if err := s.db.Where("deleted_at IS NULL").Order("id").Find(&roles).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "roles_failed", "读取角色失败", err.Error())
		return
	}
	items := make([]portalRoleSummary, 0, len(roles))
	for _, role := range roles {
		codes := []string{}
		if role.Code == "super_admin" {
			codes = rbac.AllPermissionCodes()
		} else {
			_ = s.db.Table("permissions p").Select("p.code").
				Joins("JOIN role_permissions rp ON rp.permission_id = p.id").
				Where("rp.role_id = ?", role.ID).Order("p.code").Pluck("p.code", &codes).Error
		}
		items = append(items, portalRoleSummary{
			ID: role.ID, Code: role.Code, Name: role.Name, IsSystem: role.IsSystem,
			AllowPC: role.AllowPC, AllowPDA: role.AllowPDA, PermissionCodes: codes,
		})
	}
	response.OK(c, gin.H{"items": items, "catalog": rbac.Catalog})
}

type rolePermissionUpdateRequest struct {
	PermissionCodes []string `json:"permissionCodes"`
}

func (s *Server) portalAdminUpdateRolePermissions(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	roleID, ok := parsePathUint(c, "id")
	if !ok {
		response.Fail(c, http.StatusBadRequest, "invalid_role", "角色 ID 无效", nil)
		return
	}
	var role model.Role
	if err := s.db.First(&role, roleID).Error; err != nil {
		response.Fail(c, http.StatusNotFound, "role_not_found", "角色不存在", nil)
		return
	}
	if role.Code == "super_admin" {
		response.Fail(c, http.StatusForbidden, "super_admin_locked", "超级管理员权限固定为全部权限", nil)
		return
	}
	var req rolePermissionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_request", "权限参数格式不正确", err.Error())
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		codes := uniqueStrings(req.PermissionCodes)
		if len(codes) == 0 {
			return nil
		}
		var perms []model.Permission
		if err := tx.Where("code IN ?", codes).Find(&perms).Error; err != nil {
			return err
		}
		if len(perms) != len(codes) {
			return gorm.ErrRecordNotFound
		}
		for _, permission := range perms {
			if err := tx.Create(&model.RolePermission{RoleID: roleID, PermissionID: permission.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "permission_update_failed", "权限编码不存在或保存失败", err.Error())
		return
	}
	s.audit(c, "role", roleID, "update_permissions", nil, gin.H{"permissionCodes": req.PermissionCodes})
	response.NoContent(c)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func parsePathUint(c *gin.Context, name string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	return value, err == nil && value > 0
}
