package httpapi

import (
	"errors"
	"net/http"

	"bearing-inspection/internal/httpapi/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

// respondUniqueConflict reports only the requested PostgreSQL uniqueness constraint.
func respondUniqueConflict(c *gin.Context, err error, constraint, code, message string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != constraint {
		return false
	}
	response.Fail(c, http.StatusConflict, code, message, nil)
	return true
}
