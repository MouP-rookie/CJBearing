// 文件说明：权限码校验中间件，统一返回 403。

package middleware

import (
	"net/http"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
)

// RequirePermission 只判断功能权限，数据范围和产线权限由具体处理器继续校验。
func RequirePermission(authorizer *rbac.Authorizer, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.GetUint64("user_id")
		allowed, err := authorizer.Can(uid, code, nil)
		if err != nil || !allowed {
			response.Fail(c, http.StatusForbidden, "forbidden", "没有权限执行此操作", nil)
			return
		}
		c.Next()
	}
}
