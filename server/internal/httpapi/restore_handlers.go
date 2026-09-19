package httpapi

import (
	"io"
	"net/http"
	"strings"

	"bearing-inspection/internal/httpapi/response"
	"github.com/gin-gonic/gin"
)

const backupRestoreMaxBytes = 100 << 20

func (s *Server) restoreBackup(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if scope.User.Role.Code != "super_admin" {
		response.Fail(c, http.StatusForbidden, "forbidden", "仅超级管理员可以恢复备份", nil)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, backupRestoreMaxBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "请选择 ZIP 部门备份文件", err.Error())
		return
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".zip") {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "只支持本系统生成的部门 ZIP 备份", nil)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "无法读取备份文件", err.Error())
		return
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, backupRestoreMaxBytes+1))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "读取备份文件失败", err.Error())
		return
	}
	if len(payload) > backupRestoreMaxBytes {
		response.Fail(c, http.StatusBadRequest, "file_too_large", "备份文件不能超过 100MB", nil)
		return
	}
	result, err := s.backups.RestoreDepartmentArchive(payload, c.GetUint64("user_id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "restore_failed", "恢复备份失败："+err.Error(), nil)
		return
	}
	s.audit(c, "backup", result.DepartmentID, "restore", nil, result)
	response.OK(c, result)
}
