// 文件说明：统一 API 成功、空响应和错误响应格式。

package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorBody 是前后端统一的错误响应结构，requestId 用于串联服务端日志。
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func OK(c *gin.Context, data any)      { c.JSON(http.StatusOK, data) }
func Created(c *gin.Context, data any) { c.JSON(http.StatusCreated, data) }
func NoContent(c *gin.Context)         { c.Status(http.StatusNoContent) }

// Fail 中止请求并返回结构化错误，避免 Gin 继续执行后续处理器。
func Fail(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, ErrorBody{
		Code: code, Message: message, Details: details, RequestID: c.GetString("request_id"),
	})
}
