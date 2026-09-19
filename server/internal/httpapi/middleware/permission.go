package middleware

import (
	"net/http"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
)

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
