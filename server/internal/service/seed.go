package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var adminPermissionCodes = []string{
	rbac.PermPCLogin, rbac.PermPDALogin, rbac.PermUserManage, rbac.PermProductManage,
	rbac.PermLineManage, rbac.PermShiftManage,
	rbac.PermFormView, rbac.PermFormManage, rbac.PermFormPublish, rbac.PermFormRollback,
	rbac.PermExecutionView, rbac.PermExecutionFill,
	rbac.PermSubmissionEditOwn, rbac.PermSubmissionEditScope,
	rbac.PermSubmissionVoidOwn, rbac.PermSubmissionVoidScope,
	rbac.PermLogView, rbac.PermExport, rbac.PermBackupManage, rbac.PermSystemManage,
}

var normalPermissionCodes = []string{
	rbac.PermPDALogin, rbac.PermExecutionView, rbac.PermExecutionFill,
	rbac.PermSubmissionEditOwn, rbac.PermSubmissionVoidOwn,
}

func Seed(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		permByCode := map[string]model.Permission{}
		for _, def := range rbac.Catalog {
			p := model.Permission{Code: def.Code}
			if err := tx.Where("code = ?", def.Code).FirstOrCreate(&p, model.Permission{Code: def.Code, Name: def.Name, Group: def.Group}).Error; err != nil {
				return err
			}
			p.Name, p.Group = def.Name, def.Group
			if err := tx.Save(&p).Error; err != nil {
				return err
			}
			permByCode[def.Code] = p
		}
		roles := []model.Role{
			{Code: "super_admin", Name: "超级管理员", IsSystem: true, AllowPC: true, AllowPDA: true},
			{Code: "admin", Name: "管理员", IsSystem: true, AllowPC: true, AllowPDA: true},
			{Code: "normal", Name: "普通用户", IsSystem: true, AllowPC: false, AllowPDA: true},
		}
		roleByCode := map[string]model.Role{}
		for _, spec := range roles {
			role := model.Role{Code: spec.Code}
			if err := tx.Where("code = ?", spec.Code).FirstOrCreate(&role, spec).Error; err != nil {
				return err
			}
			role.Name, role.IsSystem, role.AllowPC, role.AllowPDA = spec.Name, true, spec.AllowPC, spec.AllowPDA
			if err := tx.Save(&role).Error; err != nil {
				return err
			}
			roleByCode[spec.Code] = role
		}
		if err := replaceRolePermissions(tx, roleByCode["super_admin"].ID, rbac.AllPermissionCodes(), permByCode); err != nil {
			return err
		}
		if err := replaceRolePermissions(tx, roleByCode["admin"].ID, adminPermissionCodes, permByCode); err != nil {
			return err
		}
		if err := replaceRolePermissions(tx, roleByCode["normal"].ID, normalPermissionCodes, permByCode); err != nil {
			return err
		}

		department := model.Department{Code: "donggang"}
		if err := tx.Where("code = ?", department.Code).FirstOrCreate(&department, model.Department{Code: "donggang", Name: "东港分厂", Enabled: true}).Error; err != nil {
			return err
		}

		var superAdmin model.User
		err := tx.Joins("JOIN roles ON roles.id = users.role_id").
			Where("roles.code = ? AND roles.deleted_at IS NULL AND users.deleted_at IS NULL", "super_admin").
			First(&superAdmin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var admin model.User
			err = tx.Where("account = ?", "admin").First(&admin).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				hash, hashErr := auth.HashPassword("1234")
				if hashErr != nil {
					return hashErr
				}
				admin = model.User{Account: "admin", Name: "系统管理员", PasswordHash: hash, RoleID: roleByCode["super_admin"].ID, Enabled: true}
				if err := tx.Create(&admin).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				admin.RoleID = roleByCode["super_admin"].ID
				admin.DepartmentID = nil
				admin.Enabled = true
				if err := tx.Save(&admin).Error; err != nil {
					return err
				}
			}
		} else if err != nil {
			return err
		}
		if err := AppendProductFieldDrafts(tx); err != nil {
			return err
		}
		return nil
	})
}

func replaceRolePermissions(tx *gorm.DB, roleID uint64, codes []string, permByCode map[string]model.Permission) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
		return err
	}
	for _, code := range codes {
		p, ok := permByCode[code]
		if !ok {
			return fmt.Errorf("permission not found: %s", code)
		}
		if err := tx.Create(&model.RolePermission{RoleID: roleID, PermissionID: p.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func ResetAdminPassword(db *gorm.DB, password string) error {
	if password == "" {
		password = "1234"
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	result := db.Model(&model.User{}).Where("account = ?", "admin").Update("password_hash", hash)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("admin user not found")
	}
	return nil
}

// AppendProductFieldDrafts 为还没有产品字段的首检/末检模板批量生成新草稿版本，幂等可重复执行。
func AppendProductFieldDrafts(db *gorm.DB) error {
	var templates []model.FormTemplate
	if err := db.Where("kind IN ?", []string{model.FormKindFirstInspection, model.FormKindFinalInspection}).Find(&templates).Error; err != nil {
		return err
	}
	for _, tpl := range templates {
		var latest model.FormVersion
		if err := db.Where("template_id = ?", tpl.ID).Order("version_no desc").First(&latest).Error; err != nil {
			continue
		}
		schema, err := ParseSchema([]byte(latest.Schema))
		if err != nil || HasProductField(schema) {
			continue
		}
		raw, err := json.Marshal(PrependProductField(schema))
		if err != nil {
			return err
		}
		draft := model.FormVersion{TemplateID: tpl.ID, VersionNo: latest.VersionNo + 1, Schema: datatypes.JSON(raw), Status: model.VersionDraft, CreatedBy: tpl.CreatedBy}
		if err := db.Create(&draft).Error; err != nil {
			return err
		}
	}
	return nil
}
