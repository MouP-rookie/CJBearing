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

func (a *Authorizer) Can(userID uint64, permission string, lineID *uint64) (bool, error) {
	var user model.User
	if err := a.db.Preload("Role").First(&user, userID).Error; err != nil {
		return false, err
	}
	if !user.Enabled || user.DeletedAt.Valid {
		return false, ErrForbidden
	}
	if user.Role.DeletedAt.Valid {
		return false, ErrForbidden
	}
	if user.Role.Code == "super_admin" {
		return true, nil
	}
	var count int64
	err := a.db.Table("role_permissions rp").
		Joins("JOIN permissions p ON p.id = rp.permission_id").
		Where("rp.role_id = ? AND p.code = ?", user.RoleID, permission).
		Count(&count).Error
	if err != nil || count == 0 {
		return false, err
	}
	if lineID == nil {
		return true, nil
	}
	if user.Role.Code == "admin" || user.Role.Code == "normal" {
		return true, nil
	}
	err = a.db.Table("role_line_scopes").
		Where("role_id = ? AND line_id = ?", user.RoleID, *lineID).
		Count(&count).Error
	return count > 0, err
}

func (a *Authorizer) AllowedLines(userID uint64) ([]uint64, error) {
	var user model.User
	if err := a.db.Preload("Role").First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.Role.Code == "super_admin" || user.Role.Code == "admin" || user.Role.Code == "normal" {
		var ids []uint64
		err := a.db.Model(&model.Line{}).Where("enabled = ?", true).Pluck("id", &ids).Error
		return ids, err
	}
	var ids []uint64
	err := a.db.Model(&model.RoleLineScope{}).Where("role_id = ?", user.RoleID).Pluck("line_id", &ids).Error
	return ids, err
}
