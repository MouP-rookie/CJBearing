// 文件说明：RBAC 授权核心，统一处理权限、部门范围和产线范围判断。

package rbac

import (
	"errors"

	"bearing-inspection/internal/model"
	"gorm.io/gorm"
)

var ErrForbidden = errors.New("forbidden")

type Authorizer struct {
	db *gorm.DB
}

func NewAuthorizer(db *gorm.DB) *Authorizer { return &Authorizer{db: db} }

// Can 是统一授权入口：先校验用户和部门可用，再校验角色权限，最后叠加产线范围。
func (a *Authorizer) Can(userID uint64, permission string, lineID *uint64) (bool, error) {
	var user model.User
	if err := a.db.Preload("Role").Preload("Department").First(&user, userID).Error; err != nil {
		return false, err
	}
	if !user.Enabled || user.DeletedAt.Valid {
		return false, ErrForbidden
	}
	if user.Role.DeletedAt.Valid {
		return false, ErrForbidden
	}
	if user.Role.Code != "super_admin" && (user.DepartmentID == nil || user.Department == nil || !user.Department.Enabled || user.Department.DeletedAt.Valid) {
		return false, ErrForbidden
	}
	if user.Role.Code == "super_admin" {
		return true, nil
	}
	// 先检查角色权限；超级管理员已在上方提前放行。
	var count int64
	err := a.db.Table("role_permissions rp").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Where("rp.role_id = ? AND p.code = ?", user.RoleID, permission).
		Count(&count).Error
	if err != nil || count == 0 {
		return false, err
	}
	// 未指定产线时只校验功能权限，指定产线时继续校验部门归属和 role_line_scopes。
	if lineID == nil {
		return true, nil
	}
	var line model.Line
	if err := a.db.First(&line, *lineID).Error; err != nil {
		return false, err
	}
	if user.DepartmentID == nil || line.DepartmentID != *user.DepartmentID {
		return false, nil
	}
	if user.Role.Code == "admin" || user.Role.Code == "normal" {
		return true, nil
	}
	err = a.db.Table("role_line_scopes").
		Where("role_id = ? AND line_id = ?", user.RoleID, *lineID).
		Count(&count).Error
	return count > 0, err
}

// AllowedLines 返回当前可用的启用产线集合，超级管理员可按请求部门获取。
func (a *Authorizer) AllowedLines(userID uint64, requestedDepartmentID uint64) ([]uint64, error) {
	var user model.User
	if err := a.db.Preload("Role").Preload("Department").First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.Role.Code != "super_admin" {
		if user.DepartmentID == nil || user.Department == nil || !user.Department.Enabled || user.Department.DeletedAt.Valid {
			return []uint64{}, nil
		}
		requestedDepartmentID = *user.DepartmentID
	}
	var ids []uint64
	q := a.db.Model(&model.Line{}).Where("enabled = ? AND deleted_at IS NULL", true)
	if requestedDepartmentID > 0 {
		q = q.Where("department_id = ?", requestedDepartmentID)
	}
	if user.Role.Code == "super_admin" || user.Role.Code == "admin" || user.Role.Code == "normal" {
		err := q.Pluck("id", &ids).Error
		return ids, err
	}
	err := q.Joins("JOIN role_line_scopes rls ON rls.line_id = lines.id").
		Where("rls.role_id = ?", user.RoleID).Pluck("lines.id", &ids).Error
	return ids, err
}
