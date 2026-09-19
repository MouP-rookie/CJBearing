package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func OK(c *gin.Context, data any)      { c.JSON(http.StatusOK, data) }
func Created(c *gin.Context, data any) { c.JSON(http.StatusCreated, data) }
func NoContent(c *gin.Context)         { c.Status(http.StatusNoContent) }

func Fail(c *gin.Context, status int, code, message string, details any) {
	c.AbortWithStatusJSON(status, ErrorBody{
		Code: code, Message: message, Details: details, RequestID: c.GetString("request_id"),
	})
}
