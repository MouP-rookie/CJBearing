package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseUint64(c *gin.Context, key string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_id", "ID格式错误", nil)
		return 0, false
	}
	return id, true
}

func (s *Server) listUsers(c *gin.Context) {
	query := s.db.Preload("Role")
	if account := strings.TrimSpace(c.Query("account")); account != "" {
		query = query.Where("account ILIKE ?", "%"+account+"%")
	}
	if role := c.Query("roleId"); role != "" {
		query = query.Where("role_id = ?", role)
	}
	page, size := pageParams(c)
	var total int64
	if err := query.Model(&model.User{}).Count(&total).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询用户失败", err.Error())
		return
	}
	var users []model.User
	if err := query.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&users).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询用户失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": users, "total": total, "page": page, "size": size})
}

type userRequest struct {
	Account string `json:"account"`
	Name    string `json:"name"`
	RoleID  uint64 `json:"roleId"`
	Enabled *bool  `json:"enabled"`
}

func (s *Server) createUser(c *gin.Context) {
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	req.Account, req.Name = strings.TrimSpace(req.Account), strings.TrimSpace(req.Name)
	if req.Account == "" || req.Name == "" {
		response.Fail(c, 400, "validation_error", "账号和用户名不能为空", nil)
		return
	}
	if req.RoleID == 0 {
		response.Fail(c, 400, "validation_error", "必须选择角色", nil)
		return
	}
	role, err := s.roleByID(req.RoleID)
	if err != nil {
		response.Fail(c, 400, "validation_error", "角色不存在", nil)
		return
	}
	current, _ := s.currentUser(c)
	if current.Role.Code != "super_admin" && role.Code != "normal" {
		response.Fail(c, http.StatusForbidden, "forbidden", "管理员只能创建普通用户", nil)
		return
	}
	hash, err := auth.HashPassword("123456")
	if err != nil {
		response.Fail(c, 500, "hash_error", "生成默认密码失败", nil)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	user := model.User{Account: req.Account, Name: req.Name, PasswordHash: hash, RoleID: req.RoleID, Enabled: enabled}
	if err := s.db.Create(&user).Error; err != nil {
		response.Fail(c, 409, "create_failed", "账号可能已存在", err.Error())
		return
	}
	s.audit(c, "user", user.ID, "create", nil, user)
	response.Created(c, user)
}

func (s *Server) updateUser(c *gin.Context) {
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var user model.User
	if err := s.db.Preload("Role").First(&user, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "用户不存在", nil)
		return
	}
	current, _ := s.currentUser(c)
	if current.Role.Code != "super_admin" && user.Role.Code != "normal" {
		response.Fail(c, 403, "forbidden", "管理员只能管理普通用户", nil)
		return
	}
	var req userRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	before := user
	if strings.TrimSpace(req.Name) != "" {
		user.Name = strings.TrimSpace(req.Name)
	}
	if req.RoleID != 0 {
		role, err := s.roleByID(req.RoleID)
		if err != nil {
			response.Fail(c, 400, "validation_error", "角色不存在", nil)
			return
		}
		if current.Role.Code != "super_admin" && role.Code != "normal" {
			response.Fail(c, 403, "forbidden", "管理员只能分配普通角色", nil)
			return
		}
		if current.Role.Code == "super_admin" && user.ID == current.ID && req.RoleID != user.RoleID {
			response.Fail(c, 403, "self_role_forbidden", "不能修改自己的角色", nil)
			return
		}
		user.RoleID = req.RoleID
	}
	if req.Enabled != nil {
		if current.Role.Code == "super_admin" && user.ID == current.ID && !*req.Enabled {
			response.Fail(c, 403, "self_disable_forbidden", "不能停用当前登录账号", nil)
			return
		}
		user.Enabled = *req.Enabled
	}
	if err := s.db.Save(&user).Error; err != nil {
		response.Fail(c, 500, "save_failed", "保存用户失败", err.Error())
		return
	}
	s.audit(c, "user", user.ID, "update", before, user)
	response.OK(c, user)
}

func (s *Server) resetUserPassword(c *gin.Context) {
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var user model.User
	if err := s.db.Preload("Role").First(&user, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "用户不存在", nil)
		return
	}
	current, _ := s.currentUser(c)
	if user.Role.Code == "super_admin" && current.Role.Code != "super_admin" {
		response.Fail(c, 403, "forbidden", "不能重置超级管理员密码", nil)
		return
	}
	if current.Role.Code != "super_admin" && user.Role.Code != "normal" {
		response.Fail(c, 403, "forbidden", "管理员只能重置普通用户密码", nil)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Password == "" {
		req.Password = "123456"
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		response.Fail(c, 500, "hash_error", "生成密码失败", nil)
		return
	}
	if err := s.db.Model(&user).Update("password_hash", hash).Error; err != nil {
		response.Fail(c, 500, "save_failed", "重置密码失败", err.Error())
		return
	}
	s.audit(c, "user", user.ID, "reset_password", nil, gin.H{"account": user.Account})
	response.NoContent(c)
}

func (s *Server) listRoles(c *gin.Context) {
	var roles []model.Role
	if err := s.db.Order("id").Find(&roles).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询角色失败", err.Error())
		return
	}
	for i := range roles {
		_ = s.db.Table("permissions p").Select("p.code").Joins("JOIN role_permissions rp ON rp.permission_id = p.id").Where("rp.role_id = ?", roles[i].ID).Pluck("p.code", &[]string{}).Error
	}
	response.OK(c, gin.H{"items": roles})
}

func (s *Server) listRoleOptions(c *gin.Context) {
	current, err := s.currentUser(c)
	if err != nil {
		response.Fail(c, 401, "unauthorized", "当前用户不存在", nil)
		return
	}
	q := s.db.Order("id")
	if current.Role.Code != "super_admin" {
		q = q.Where("code = ?", "normal")
	}
	var roles []model.Role
	if err := q.Find(&roles).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询角色失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": roles})
}

type roleRequest struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	AllowPC         bool     `json:"allowPc"`
	AllowPDA        bool     `json:"allowPda"`
	PermissionCodes []string `json:"permissionCodes"`
	LineIDs         []uint64 `json:"lineIds"`
}

func (s *Server) createRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		response.Fail(c, 400, "validation_error", "角色编码和名称不能为空", nil)
		return
	}
	role := model.Role{Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name), AllowPC: req.AllowPC, AllowPDA: req.AllowPDA}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRoleGrants(tx, role.ID, req.PermissionCodes, req.LineIDs)
	})
	if err != nil {
		response.Fail(c, 409, "create_failed", "创建角色失败", err.Error())
		return
	}
	s.audit(c, "role", role.ID, "create", nil, role)
	response.Created(c, role)
}

func (s *Server) updateRole(c *gin.Context) {
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var role model.Role
	if err := s.db.First(&role, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "角色不存在", nil)
		return
	}
	if role.Code == "super_admin" {
		response.Fail(c, 403, "forbidden", "不能修改超级管理员角色", nil)
		return
	}
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	before := role
	if strings.TrimSpace(req.Name) != "" {
		role.Name = strings.TrimSpace(req.Name)
	}
	role.AllowPC, role.AllowPDA = req.AllowPC, req.AllowPDA
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&role).Error; err != nil {
			return err
		}
		return replaceRoleGrants(tx, role.ID, req.PermissionCodes, req.LineIDs)
	})
	if err != nil {
		response.Fail(c, 500, "save_failed", "保存角色失败", err.Error())
		return
	}
	s.audit(c, "role", role.ID, "update", before, role)
	response.OK(c, role)
}

func replaceRoleGrants(tx *gorm.DB, roleID uint64, permissionCodes []string, lineIDs []uint64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
		return err
	}
	if len(permissionCodes) > 0 {
		var perms []model.Permission
		if err := tx.Where("code IN ?", permissionCodes).Find(&perms).Error; err != nil {
			return err
		}
		for _, p := range perms {
			if err := tx.Create(&model.RolePermission{RoleID: roleID, PermissionID: p.ID}).Error; err != nil {
				return err
			}
		}
	}
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleLineScope{}).Error; err != nil {
		return err
	}
	for _, lineID := range lineIDs {
		if err := tx.Create(&model.RoleLineScope{RoleID: roleID, LineID: lineID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) roleByID(id uint64) (*model.Role, error) {
	var r model.Role
	return &r, s.db.First(&r, id).Error
}

func (s *Server) listLines(c *gin.Context) {
	var lines []model.Line
	if err := s.db.Order("level, id").Find(&lines).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": lines})
}

type lineRequest struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *uint64 `json:"parentId"`
	Enabled  *bool   `json:"enabled"`
}

func (s *Server) createLine(c *gin.Context) {
	var req lineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	level := 1
	if req.ParentID != nil {
		var parent model.Line
		if err := s.db.First(&parent, *req.ParentID).Error; err != nil {
			response.Fail(c, 400, "validation_error", "父产线不存在", nil)
			return
		}
		if parent.ParentID != nil {
			response.Fail(c, 400, "validation_error", "产线最多两级", nil)
			return
		}
		level = 2
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	line := model.Line{Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name), ParentID: req.ParentID, Level: level, Enabled: enabled}
	if err := s.db.Create(&line).Error; err != nil {
		response.Fail(c, 409, "create_failed", "产线编码可能已存在", err.Error())
		return
	}
	s.audit(c, "line", line.ID, "create", nil, line)
	response.Created(c, line)
}

func (s *Server) updateLine(c *gin.Context) {
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var line model.Line
	if err := s.db.First(&line, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "产线不存在", nil)
		return
	}
	var req lineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	before := line
	if strings.TrimSpace(req.Name) != "" {
		line.Name = strings.TrimSpace(req.Name)
	}
	if req.Enabled != nil {
		line.Enabled = *req.Enabled
	}
	if err := s.db.Save(&line).Error; err != nil {
		response.Fail(c, 500, "save_failed", "保存产线失败", err.Error())
		return
	}
	s.audit(c, "line", line.ID, "update", before, line)
	response.OK(c, line)
}

func (s *Server) deleteLine(c *gin.Context) {
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var children int64
	s.db.Model(&model.Line{}).Where("parent_id = ?", id).Count(&children)
	if children > 0 {
		response.Fail(c, 409, "has_children", "请先删除或停用子产线", nil)
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("line_id = ?", id).Delete(&model.LineFormAssignment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Line{}, id).Error
	}); err != nil {
		response.Fail(c, 500, "delete_failed", "删除失败", err.Error())
		return
	}
	s.audit(c, "line", id, "delete", nil, nil)
	response.NoContent(c)
}

func (s *Server) listShiftPlans(c *gin.Context) {
	var plans []model.ShiftPlan
	if err := s.db.Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order, id") }).Order("effective_at desc").Find(&plans).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询班次失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": plans})
}

type shiftPlanRequest struct {
	Name        string             `json:"name"`
	EffectiveAt string             `json:"effectiveAt"`
	Items       []shiftItemRequest `json:"items"`
}

type shiftItemRequest struct {
	Name          string `json:"name"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	CrossMidnight bool   `json:"crossMidnight"`
	SortOrder     int    `json:"sortOrder"`
}

func (s *Server) createShiftPlan(c *gin.Context) {
	var req shiftPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	plan, err := buildShiftPlan(req, c.GetUint64("user_id"))
	if err != nil {
		response.Fail(c, 400, "validation_error", err.Error(), nil)
		return
	}
	if err := s.db.Create(plan).Error; err != nil {
		response.Fail(c, 500, "create_failed", "创建班次失败", err.Error())
		return
	}
	s.audit(c, "shift_plan", plan.ID, "create", nil, plan)
	response.Created(c, plan)
}

func (s *Server) updateShiftPlan(c *gin.Context) {
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var existing model.ShiftPlan
	if err := s.db.First(&existing, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "班次计划不存在", nil)
		return
	}
	var req shiftPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	replacement, err := buildShiftPlan(req, c.GetUint64("user_id"))
	if err != nil {
		response.Fail(c, 400, "validation_error", err.Error(), nil)
		return
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(replacement).Error; err != nil {
			return err
		}
		return tx.Delete(&existing).Error
	})
	if err != nil {
		response.Fail(c, 500, "update_failed", "修改班次失败", err.Error())
		return
	}
	s.audit(c, "shift_plan", replacement.ID, "update", existing, replacement)
	response.OK(c, replacement)
}

func (s *Server) deleteShiftPlan(c *gin.Context) {
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var plan model.ShiftPlan
	if err := s.db.First(&plan, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "班次计划不存在", nil)
		return
	}
	if err := s.db.Delete(&plan).Error; err != nil {
		response.Fail(c, 500, "delete_failed", "删除班次失败", err.Error())
		return
	}
	s.audit(c, "shift_plan", plan.ID, "delete", plan, nil)
	response.NoContent(c)
}

func buildShiftPlan(req shiftPlanRequest, userID uint64) (*model.ShiftPlan, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("至少配置一个班次")
	}
	effective := time.Now()
	if strings.TrimSpace(req.EffectiveAt) != "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(req.EffectiveAt), loc)
		if err != nil {
			return nil, fmt.Errorf("生效时间格式应为 YYYY-MM-DD HH:mm:ss")
		}
		effective = parsed
	}
	plan := &model.ShiftPlan{Name: strings.TrimSpace(req.Name), EffectiveAt: effective, CreatedBy: userID}
	if plan.Name == "" {
		plan.Name = "班次计划"
	}
	for _, item := range req.Items {
		if strings.TrimSpace(item.Name) == "" {
			return nil, fmt.Errorf("班次名称不能为空")
		}
		if _, err := time.Parse("15:04", item.StartTime); err != nil {
			return nil, fmt.Errorf("班次开始时间格式应为 HH:mm")
		}
		if _, err := time.Parse("15:04", item.EndTime); err != nil {
			return nil, fmt.Errorf("班次结束时间格式应为 HH:mm")
		}
		plan.Items = append(plan.Items, model.ShiftPlanItem{Name: strings.TrimSpace(item.Name), StartTime: item.StartTime, EndTime: item.EndTime, CrossMidnight: item.CrossMidnight, SortOrder: item.SortOrder})
	}
	return plan, nil
}
func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return page, size
}

func (s *Server) audit(c *gin.Context, entity string, id uint64, action string, before, after any) {
	beforeJSON := mustJSON(before)
	afterJSON := mustJSON(after)
	_ = s.db.Create(&model.AuditLog{EntityType: entity, EntityID: id, Action: action, BeforeJSON: beforeJSON, AfterJSON: afterJSON, ActorID: c.GetUint64("user_id"), IP: c.ClientIP(), RequestID: c.GetString("request_id")}).Error
}

func mustJSON(v any) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

var _ = errors.Is
var _ = fmt.Sprintf
