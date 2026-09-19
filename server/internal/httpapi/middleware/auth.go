package middleware

import (
	"net/http"
	"strings"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/httpapi/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func Auth(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := ""
		authz := c.GetHeader("Authorization")
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			raw = strings.TrimSpace(authz[7:])
		}
		if raw == "" {
			if cookie, err := c.Cookie("token"); err == nil {
				raw = cookie
			}
		}
		if raw == "" {
			response.Fail(c, http.StatusUnauthorized, "unauthorized", "请先登录", nil)
			return
		}
		claims, err := svc.ParseToken(raw)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "unauthorized", "登录已失效", nil)
			return
		}
		c.Set("claims", claims)
		c.Set("user_id", claims.UserID)
		c.Set("client", claims.Client)
		c.Next()
	}
}

func RequireClient(client string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("client") != client {
			response.Fail(c, http.StatusForbidden, "wrong_client", "当前账号不能在此客户端登录", nil)
			return
		}
		c.Next()
	}
}
