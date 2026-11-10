// 文件说明：创建首末检账号时，在同一事务内写入默认系统入口授权。
package service

import (
	"bearing-inspection/internal/model"
	"gorm.io/gorm"
)

// CreateInspectionUser 将账号和首末检入口授权一起保存，避免新账号被门户拦截。
// 此处只赋予首末检入口；客户端和功能权限仍由角色控制。
func CreateInspectionUser(db *gorm.DB, user *model.User, actorID uint64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO portal_system_access (user_id, system_code, enabled, updated_by, updated_at)
   VALUES (?, 'bearing_inspection', TRUE, ?, NOW())
   ON CONFLICT (user_id, system_code) DO NOTHING`, user.ID, actorID).Error
	})
}
