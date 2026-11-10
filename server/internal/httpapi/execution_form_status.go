package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// executionFormStatus returns submitted templates for the selected or current shift.
func (s *Server) executionFormStatus(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, err := strconv.ParseUint(c.Query("lineId"), 10, 64)
	var line model.Line
	if err != nil || lineID == 0 || !s.canLine(scope.User.ID, rbac.PermExecutionView, lineID) ||
		scope.Filter(s.db, "department_id").First(&line, lineID).Error != nil {
		response.Fail(c, http.StatusForbidden, "forbidden", "没有该产线权限", nil)
		return
	}

	date, shift := c.Query("businessDate"), c.Query("shiftPlanItemId")
	var shiftID uint64
	if date == "" && shift == "" {
		plan, err := service.ActiveShiftPlan(s.db, line.DepartmentID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.OK(c, gin.H{"submittedTemplateIds": nil})
			return
		}
		if err != nil {
			response.Fail(c, 500, "db_error", "读取班次计划失败", nil)
			return
		}
		current, err := service.CalculateShift(*plan, time.Now())
		if errors.Is(err, service.ErrShiftNotFound) {
			response.OK(c, gin.H{"submittedTemplateIds": nil})
			return
		}
		if err != nil {
			response.Fail(c, 500, "shift_unavailable", "无法计算当前班次", nil)
			return
		}
		date, shiftID = current.BusinessDate.Format(time.DateOnly), current.Item.ID
	} else {
		var err error
		shiftID, err = strconv.ParseUint(shift, 10, 64)
		if _, dateErr := service.ParseBusinessTime(date); err != nil || dateErr != nil || shiftID == 0 {
			response.Fail(c, 400, "validation_error", "请选择有效的业务日期和班次", nil)
			return
		}
		var count int64
		err = s.db.Table("shift_plan_items i").
			Joins("JOIN shift_plans p ON p.id = i.plan_id").
			Where("i.id = ? AND p.department_id = ? AND p.deleted_at IS NULL", shiftID, line.DepartmentID).
			Count(&count).Error
		if err != nil {
			response.Fail(c, 500, "db_error", "读取班次失败", nil)
			return
		}
		if count == 0 {
			response.Fail(c, 400, "validation_error", "班次不存在或不属于该产线部门", nil)
			return
		}
	}

	templateIDs := make([]uint64, 0)
	err = s.db.Model(&model.Submission{}).
		Where("department_id = ? AND line_id = ? AND business_date = ? AND shift_plan_item_id = ?", line.DepartmentID, lineID, date, shiftID).
		Where("EXISTS (SELECT 1 FROM submission_rows r WHERE r.submission_id = submissions.id AND r.status = ? AND r.deleted_at IS NULL)", model.SubmissionStatusActive).
		Distinct("template_id").Pluck("template_id", &templateIDs).Error
	if err != nil {
		response.Fail(c, 500, "db_error", "查询当班填报状态失败", nil)
		return
	}
	response.OK(c, gin.H{"submittedTemplateIds": templateIDs, "businessDate": date, "shiftPlanItemId": shiftID})
}
