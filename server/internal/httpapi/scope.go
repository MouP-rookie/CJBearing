package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type departmentScope struct {
	User           *model.User
	DepartmentID   uint64
	AllDepartments bool
}

func (s *Server) scopeOrFail(c *gin.Context) (*departmentScope, bool) {
	user, err := s.currentUser(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, "unauthorized", "用户不存在或已停用", nil)
		return nil, false
	}
	scope := &departmentScope{User: user}
	if user.Role.Code == "super_admin" {
		raw := strings.TrimSpace(c.GetHeader("X-Department-ID"))
		if raw == "" || raw == "0" || raw == "all" {
			scope.AllDepartments = true
		} else {
			id, err := strconv.ParseUint(raw, 10, 64)
			if err != nil || id == 0 {
				response.Fail(c, http.StatusBadRequest, "invalid_department", "部门参数无效", nil)
				return nil, false
			}
			var department model.Department
			if err := s.db.First(&department, id).Error; err != nil {
				scope.AllDepartments = true
			} else {
				scope.DepartmentID = department.ID
			}
		}
	} else {
		if user.DepartmentID == nil || user.Department == nil || !user.Department.Enabled || user.Department.DeletedAt.Valid {
			response.Fail(c, http.StatusForbidden, "department_disabled", "所属部门不存在或已停用", nil)
			return nil, false
		}
		scope.DepartmentID = *user.DepartmentID
	}
	c.Set("scope_department_id", scope.DepartmentID)
	c.Set("scope_all_departments", scope.AllDepartments)
	return scope, true
}

func (s *Server) requireSuperAdmin(c *gin.Context) bool {
	user, err := s.currentUser(c)
	if err != nil || user.Role.Code != "super_admin" {
		response.Fail(c, http.StatusForbidden, "forbidden", "仅超级管理员可以执行此操作", nil)
		return false
	}
	return true
}

func (s *departmentScope) Filter(query *gorm.DB, column string) *gorm.DB {
	if s.AllDepartments {
		return query
	}
	return query.Where(column+" = ?", s.DepartmentID)
}

func (s *departmentScope) FilterOptional(query *gorm.DB, column string) *gorm.DB {
	if s.AllDepartments {
		return query
	}
	return query.Where(column+" IS NOT NULL AND "+column+" = ?", s.DepartmentID)
}

func (s *departmentScope) CanAccessDepartment(departmentID uint64) bool {
	return s.AllDepartments || s.DepartmentID == departmentID
}

func (s *departmentScope) WritableDepartmentID(db *gorm.DB) (uint64, error) {
	if s.AllDepartments || s.DepartmentID == 0 {
		return 0, errors.New("请先选择部门")
	}
	var department model.Department
	if err := db.First(&department, s.DepartmentID).Error; err != nil {
		return 0, err
	}
	if !department.Enabled || department.DeletedAt.Valid {
		return 0, errors.New("部门已停用，不能执行修改操作")
	}
	return s.DepartmentID, nil
}
