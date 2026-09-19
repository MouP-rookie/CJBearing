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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	query := s.db.Preload("Role").Preload("Department")
	if scope.AllDepartments {
		// Super administrators can inspect every department.
	} else {
		query = query.Where("department_id = ?", scope.DepartmentID)
	}
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
	Account      string  `json:"account"`
	Name         string  `json:"name"`
	RoleID       uint64  `json:"roleId"`
	DepartmentID *uint64 `json:"departmentId"`
	Enabled      *bool   `json:"enabled"`
}

func (s *Server) createUser(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if scope.AllDepartments {
		response.Fail(c, http.StatusBadRequest, "department_required", "请先选择部门", nil)
		return
	}
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
	if role.Code == "super_admin" {
		response.Fail(c, http.StatusForbidden, "super_admin_create_forbidden", "系统只允许一个超级管理员，不能创建或分配超级管理员角色", nil)
		return
	}
	departmentID, err := s.resolveUserDepartment(scope, req.DepartmentID, role.Code)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
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
	user := model.User{Account: req.Account, Name: req.Name, PasswordHash: hash, RoleID: req.RoleID, DepartmentID: departmentID, Enabled: enabled}
	if err := s.db.Create(&user).Error; err != nil {
		response.Fail(c, 409, "create_failed", "账号可能已存在", err.Error())
		return
	}
	s.audit(c, "user", user.ID, "create", nil, user)
	response.Created(c, user)
}

func (s *Server) updateUser(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if scope.AllDepartments {
		response.Fail(c, http.StatusBadRequest, "department_required", "请先选择部门", nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var user model.User
	query := s.db.Preload("Role").Preload("Department")
	if !scope.AllDepartments {
		query = query.Where("department_id = ?", scope.DepartmentID)
	}
	if err := query.First(&user, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "用户不存在", nil)
		return
	}
	current := scope.User
	if current.Role.Code != "super_admin" && user.Role.Code == "super_admin" {
		response.Fail(c, http.StatusNotFound, "not_found", "用户不存在", nil)
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
	nextRole := user.Role
	if req.RoleID != 0 {
		role, err := s.roleByID(req.RoleID)
		if err != nil {
			response.Fail(c, 400, "validation_error", "角色不存在", nil)
			return
		}
		if role.Code == "super_admin" {
			response.Fail(c, http.StatusForbidden, "super_admin_assign_forbidden", "系统只允许一个超级管理员，不能创建或分配超级管理员角色", nil)
			return
		}
		if user.ID == current.ID && req.RoleID != user.RoleID {
			response.Fail(c, 403, "self_role_forbidden", "不能修改自己的角色", nil)
			return
		}
		nextRole = *role
		user.RoleID = req.RoleID
	}
	if current.Role.Code == "super_admin" {
		if nextRole.Code == "super_admin" {
			user.DepartmentID = nil
		} else if req.DepartmentID != nil || req.RoleID != 0 {
			departmentID, err := s.resolveUserDepartment(scope, req.DepartmentID, nextRole.Code)
			if err != nil {
				response.Fail(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
				return
			}
			user.DepartmentID = departmentID
		} else if user.DepartmentID == nil {
			response.Fail(c, http.StatusBadRequest, "validation_error", "非超级管理员必须选择部门", nil)
			return
		}
	} else {
		user.DepartmentID = current.DepartmentID
	}
	if req.Enabled != nil {
		if user.ID == current.ID && !*req.Enabled {
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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if scope.AllDepartments {
		response.Fail(c, http.StatusBadRequest, "department_required", "请先选择部门", nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var user model.User
	query := s.db.Preload("Role").Preload("Department")
	if !scope.AllDepartments {
		query = query.Where("department_id = ?", scope.DepartmentID)
	}
	if err := query.First(&user, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "用户不存在", nil)
		return
	}
	current := scope.User
	if user.Role.Code == "super_admin" && current.Role.Code != "super_admin" {
		response.Fail(c, 403, "forbidden", "不能重置超级管理员密码", nil)
		return
	}
	if current.Role.Code != "super_admin" && user.Role.Code == "super_admin" {
		response.Fail(c, http.StatusNotFound, "not_found", "用户不存在", nil)
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
	q := s.db.Order("id")
	q = q.Where("code <> ?", "super_admin")
	var roles []model.Role
	if err := q.Find(&roles).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询角色失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": roles})
}

func (s *Server) resolveUserDepartment(scope *departmentScope, requested *uint64, roleCode string) (*uint64, error) {
	if roleCode == "super_admin" {
		return nil, fmt.Errorf("系统只允许一个超级管理员，不能创建或分配超级管理员角色")
	}
	if scope.User.Role.Code != "super_admin" {
		if scope.User.DepartmentID == nil {
			return nil, fmt.Errorf("当前管理员未绑定部门")
		}
		return scope.User.DepartmentID, nil
	}
	departmentID := scope.DepartmentID
	if requested != nil && *requested > 0 {
		departmentID = *requested
	}
	if departmentID == 0 {
		return nil, fmt.Errorf("非超级管理员必须选择部门")
	}
	var department model.Department
	if err := s.db.First(&department, departmentID).Error; err != nil {
		return nil, fmt.Errorf("部门不存在")
	}
	if !department.Enabled || department.DeletedAt.Valid {
		return nil, fmt.Errorf("部门已停用")
	}
	return &department.ID, nil
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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	page, size := pageParams(c)
	var allRoots []model.Line
	if err := scope.Filter(s.db.Preload("Department"), "department_id").Where("level = ?", 1).Find(&allRoots).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	sortLinesByDepartmentAndCode(allRoots)
	total := int64(len(allRoots))
	start := (page - 1) * size
	if start > len(allRoots) {
		start = len(allRoots)
	}
	end := start + size
	if end > len(allRoots) {
		end = len(allRoots)
	}
	roots := allRoots[start:end]
	items := append([]model.Line(nil), roots...)
	if len(roots) > 0 {
		rootIDs := make([]uint64, 0, len(roots))
		rootByID := make(map[uint64]model.Line, len(roots))
		for _, root := range roots {
			rootIDs = append(rootIDs, root.ID)
			rootByID[root.ID] = root
		}
		var children []model.Line
		if err := s.db.Preload("Department").Where("parent_id IN ?", rootIDs).Order("parent_id, code, id").Find(&children).Error; err != nil {
			response.Fail(c, 500, "db_error", "查询子产线失败", err.Error())
			return
		}
		filtered := children[:0]
		for _, child := range children {
			if child.ParentID == nil {
				continue
			}
			if root, ok := rootByID[*child.ParentID]; ok && root.DepartmentID == child.DepartmentID {
				filtered = append(filtered, child)
			}
		}
		children = filtered
		sortLinesByCode(children)
		items = append(items, children...)
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "size": size})
}

type lineRequest struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	ParentID *uint64 `json:"parentId"`
	Enabled  *bool   `json:"enabled"`
}

func (s *Server) createLine(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, err := scope.WritableDepartmentID(s.db)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	var req lineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	level := 1
	if req.ParentID != nil {
		var parent model.Line
		if err := s.db.Where("department_id = ?", departmentID).First(&parent, *req.ParentID).Error; err != nil {
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
	line := model.Line{DepartmentID: departmentID, Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name), ParentID: req.ParentID, Level: level, Enabled: enabled}
	if err := s.db.Create(&line).Error; err != nil {
		response.Fail(c, 409, "create_failed", "产线编码可能已存在", err.Error())
		return
	}
	s.audit(c, "line", line.ID, "create", nil, line)
	response.Created(c, line)
}

func (s *Server) updateLine(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var line model.Line
	if err := scope.Filter(s.db, "department_id").First(&line, id).Error; err != nil {
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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var line model.Line
	if err := scope.Filter(s.db, "department_id").First(&line, id).Error; err != nil {
		response.Fail(c, http.StatusNotFound, "not_found", "产线不存在", nil)
		return
	}
	var children int64
	s.db.Model(&model.Line{}).Where("department_id = ? AND parent_id = ?", line.DepartmentID, id).Count(&children)
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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	page, size := pageParams(c)
	var total int64
	if err := scope.Filter(s.db.Model(&model.ShiftPlan{}), "department_id").Count(&total).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询班次失败", err.Error())
		return
	}
	var plans []model.ShiftPlan
	if err := scope.Filter(s.db.Preload("Department").Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order, id") }), "department_id").
		Order("department_id, effective_at desc").Offset((page - 1) * size).Limit(size).Find(&plans).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询班次失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": plans, "total": total, "page": page, "size": size})
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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, err := scope.WritableDepartmentID(s.db)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	var req shiftPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	plan, err := buildShiftPlan(req, c.GetUint64("user_id"), departmentID)
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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var existing model.ShiftPlan
	if err := scope.Filter(s.db, "department_id").First(&existing, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "班次计划不存在", nil)
		return
	}
	var req shiftPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	replacement, err := buildShiftPlan(req, c.GetUint64("user_id"), existing.DepartmentID)
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
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var plan model.ShiftPlan
	if err := scope.Filter(s.db, "department_id").First(&plan, id).Error; err != nil {
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

func buildShiftPlan(req shiftPlanRequest, userID, departmentID uint64) (*model.ShiftPlan, error) {
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
	plan := &model.ShiftPlan{DepartmentID: departmentID, Name: strings.TrimSpace(req.Name), EffectiveAt: effective, CreatedBy: userID}
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
	var departmentID *uint64
	if id := c.GetUint64("scope_department_id"); id > 0 {
		departmentID = &id
	}
	_ = s.db.Create(&model.AuditLog{DepartmentID: departmentID, EntityType: entity, EntityID: id, Action: action, BeforeJSON: beforeJSON, AfterJSON: afterJSON, ActorID: c.GetUint64("user_id"), IP: c.ClientIP(), RequestID: c.GetString("request_id")}).Error
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
