// 文件说明：校验门户层系统查看权限；超级管理员默认拥有全部系统。

package middleware

import (
	"net/http"

	"bearing-inspection/internal/httpapi/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RequireSystemAccess(db *gorm.DB, systemCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("user_id")
		if userID == 0 {
			response.Fail(c, http.StatusUnauthorized, "unauthorized", "请先登录", nil)
			return
		}
		var isSuperAdmin bool
		if err := db.Table("users u").
			Select("EXISTS (SELECT 1 FROM roles r WHERE r.id = u.role_id AND r.code = 'super_admin' AND r.deleted_at IS NULL)").
			Where("u.id = ? AND u.deleted_at IS NULL", userID).
			Scan(&isSuperAdmin).Error; err != nil {
			response.Fail(c, http.StatusInternalServerError, "system_access_failed", "系统权限校验失败", nil)
			return
		}
		if isSuperAdmin {
			c.Next()
			return
		}
		var count int64
		if err := db.Table("portal_system_access").
			Where("user_id = ? AND system_code = ? AND enabled = ?", userID, systemCode, true).
			Count(&count).Error; err != nil || count == 0 {
			response.Fail(c, http.StatusForbidden, "system_forbidden", "没有该系统的查看权限", nil)
			return
		}
		c.Next()
	}
}
