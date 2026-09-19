package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) listDepartments(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	page, size := pageParams(c)
	var total int64
	if err := s.db.Model(&model.Department{}).Count(&total).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "db_error", "查询部门失败", err.Error())
		return
	}
	var departments []model.Department
	if err := s.db.Order("code, id").Offset((page - 1) * size).Limit(size).Find(&departments).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "db_error", "查询部门失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": departments, "total": total, "page": page, "size": size})
}

type departmentRequest struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Enabled *bool  `json:"enabled"`
}

func (s *Server) createDepartment(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_request", "参数错误", err.Error())
		return
	}
	req.Code, req.Name = strings.TrimSpace(req.Code), strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		response.Fail(c, http.StatusBadRequest, "validation_error", "部门编码和名称不能为空", nil)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	department := model.Department{Code: req.Code, Name: req.Name, Enabled: enabled}
	if err := s.db.Create(&department).Error; err != nil {
		response.Fail(c, http.StatusConflict, "create_failed", "部门编码可能已存在", err.Error())
		return
	}
	s.audit(c, "department", department.ID, "create", nil, department)
	response.Created(c, department)
}

func (s *Server) updateDepartment(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var department model.Department
	if err := s.db.First(&department, id).Error; err != nil {
		response.Fail(c, http.StatusNotFound, "not_found", "部门不存在", nil)
		return
	}
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_request", "参数错误", err.Error())
		return
	}
	before := department
	if name := strings.TrimSpace(req.Name); name != "" {
		department.Name = name
	}
	if req.Enabled != nil {
		department.Enabled = *req.Enabled
	}
	if err := s.db.Save(&department).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "save_failed", "保存部门失败", err.Error())
		return
	}
	s.audit(c, "department", department.ID, "update", before, department)
	response.OK(c, department)
}

func (s *Server) deleteDepartment(c *gin.Context) {
	if !s.requireSuperAdmin(c) {
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var department model.Department
	if err := s.db.First(&department, id).Error; err != nil {
		response.Fail(c, http.StatusNotFound, "not_found", "部门不存在", nil)
		return
	}
	checks := []any{
		&model.User{}, &model.Line{}, &model.Product{}, &model.ShiftPlan{},
		&model.FormTemplate{}, &model.Submission{}, &model.BackupRecord{},
		&model.AuditLog{}, &model.InstrumentMessage{},
	}
	for _, item := range checks {
		var count int64
		if err := s.db.Model(item).Where("department_id = ?", id).Count(&count).Error; err != nil {
			response.Fail(c, http.StatusInternalServerError, "db_error", "检查部门数据失败", err.Error())
			return
		}
		if count > 0 {
			response.Fail(c, http.StatusConflict, "department_in_use", "部门已有数据或人员，请停用而不是删除", gin.H{"count": count})
			return
		}
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("department_id = ?", id).Delete(&model.DepartmentSetting{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Delete(&department).Error
	}); err != nil {
		response.Fail(c, http.StatusInternalServerError, "delete_failed", "删除部门失败", err.Error())
		return
	}
	s.audit(c, "department", id, "delete", department, nil)
	response.NoContent(c)
}
