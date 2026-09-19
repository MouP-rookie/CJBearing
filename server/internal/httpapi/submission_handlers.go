package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type valueInput struct {
	FieldKey    string   `json:"fieldKey"`
	ValueType   string   `json:"valueType"`
	ValueText   string   `json:"valueText"`
	ValueNumber *float64 `json:"valueNumber"`
	RepeatIndex int      `json:"repeatIndex"`
	Source      string   `json:"source"`
}
type rowInput struct {
	RowNo      int          `json:"rowNo"`
	OperatorID uint64       `json:"operatorId"`
	OperatedAt *time.Time   `json:"operatedAt"`
	Values     []valueInput `json:"values"`
}
type submissionRequest struct {
	TemplateID      uint64     `json:"templateId"`
	VersionID       uint64     `json:"versionId"`
	LineID          uint64     `json:"lineId"`
	BusinessDate    string     `json:"businessDate"`
	ShiftPlanItemID uint64     `json:"shiftPlanItemId"`
	RepeatReason    string     `json:"repeatReason"`
	Source          string     `json:"source"`
	ClientTime      *time.Time `json:"clientTime"`
	Rows            []rowInput `json:"rows"`
}

func (s *Server) createSubmission(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, scopeErr := scope.WritableDepartmentID(s.db)
	if scopeErr != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", scopeErr.Error(), nil)
		return
	}
	var req submissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	uid := c.GetUint64("user_id")
	if !s.canLineInScope(scope, rbac.PermExecutionFill, req.LineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	var version model.FormVersion
	if err := s.db.First(&version, req.VersionID).Error; err != nil {
		response.Fail(c, 404, "not_found", "表单版本不存在", nil)
		return
	}
	if version.TemplateID != req.TemplateID {
		response.Fail(c, 400, "validation_error", "模板和版本不匹配", nil)
		return
	}
	var tpl model.FormTemplate
	if err := s.db.Where("department_id = ?", departmentID).First(&tpl, req.TemplateID).Error; err != nil {
		response.Fail(c, 404, "not_found", "模板不存在", nil)
		return
	}
	var shift model.ShiftPlanItem
	if err := s.db.First(&shift, req.ShiftPlanItemID).Error; err != nil {
		response.Fail(c, 400, "validation_error", "班次不存在", nil)
		return
	}
	var shiftPlan model.ShiftPlan
	if err := s.db.Where("department_id = ?", departmentID).First(&shiftPlan, shift.PlanID).Error; err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", "班次不属于当前部门", nil)
		return
	}
	if err := s.ensureLinesInDepartment([]uint64{req.LineID}, departmentID); err != nil {
		response.Fail(c, http.StatusForbidden, "forbidden", "没有该产线权限", nil)
		return
	}
	operatorIDs := make([]uint64, 0, len(req.Rows))
	for _, rowInput := range req.Rows {
		if rowInput.OperatorID != 0 {
			operatorIDs = append(operatorIDs, rowInput.OperatorID)
		}
	}
	if err := s.ensureUsersInDepartment(operatorIDs, departmentID); err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", "操作人员不属于当前部门", nil)
		return
	}
	businessDate, err := service.ParseBusinessTime(req.BusinessDate)
	if err != nil {
		response.Fail(c, 400, "validation_error", "业务日期格式应为YYYY-MM-DD", nil)
		return
	}
	if tpl.Kind == model.FormKindFirstInspection {
		var count int64
		s.db.Model(&model.Submission{}).Where("template_id = ? AND line_id = ? AND business_date = ? AND shift_plan_item_id = ?", req.TemplateID, req.LineID, businessDate, req.ShiftPlanItemID).Count(&count)
		if count > 0 && strings.TrimSpace(req.RepeatReason) == "" {
			response.Fail(c, 400, "repeat_reason_required", "重复首检必须填写首检原因", nil)
			return
		}
	}
	if len(req.Rows) == 0 {
		req.Rows = []rowInput{{RowNo: 1, OperatorID: uid, Values: []valueInput{}}}
	}
	submittedAt := time.Now()
	var created model.Submission
	err = s.db.Transaction(func(tx *gorm.DB) error {
		created = model.Submission{DepartmentID: departmentID, TemplateID: req.TemplateID, VersionID: req.VersionID, LineID: req.LineID, BusinessDate: businessDate, ShiftPlanItemID: shift.ID, ShiftNameSnapshot: shift.Name, ShiftStartSnapshot: shift.StartTime, ShiftEndSnapshot: shift.EndTime, RepeatReason: strings.TrimSpace(req.RepeatReason), Source: req.Source, ClientTime: req.ClientTime, CreatedBy: uid, SubmittedAt: submittedAt}
		if created.Source == "" {
			created.Source = "online"
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		for i, rowInput := range req.Rows {
			if rowInput.RowNo == 0 {
				rowInput.RowNo = i + 1
			}
			row := model.SubmissionRow{SubmissionID: created.ID, RowNo: rowInput.RowNo, OperatorID: uid, OperatedAt: submittedAt, Status: model.SubmissionStatusActive}
			if rowInput.OperatorID != 0 {
				row.OperatorID = rowInput.OperatorID
			}
			if rowInput.OperatedAt != nil {
				row.OperatedAt = *rowInput.OperatedAt
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			for _, val := range rowInput.Values {
				if err := tx.Create(&model.FieldValue{SubmissionID: created.ID, RowID: &row.ID, FieldKey: val.FieldKey, ValueType: val.ValueType, ValueText: val.ValueText, ValueNumber: val.ValueNumber, RepeatIndex: val.RepeatIndex, Source: defaultSource(val.Source), CreatedBy: uid}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, 500, "submit_failed", "提交失败", err.Error())
		return
	}
	s.audit(c, "submission", created.ID, "create", nil, created)
	response.Created(c, created)
}

func (s *Server) listSubmissions(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	if lineID == 0 || !s.canLineInScope(scope, rbac.PermExecutionView, lineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	q := s.db.Table("submissions s").Select("s.id,s.template_id,s.version_id,s.line_id,s.business_date,s.shift_plan_item_id,s.shift_name_snapshot,s.repeat_reason,s.created_by,s.submitted_at").Where("s.line_id = ? AND s.deleted_at IS NULL", lineID)
	if v := c.Query("date"); v != "" {
		q = q.Where("s.business_date = ?", v)
	}
	if v := c.Query("dateFrom"); v != "" {
		q = q.Where("s.business_date >= ?", v)
	}
	if v := c.Query("dateTo"); v != "" {
		q = q.Where("s.business_date <= ?", v)
	}
	if v := c.Query("templateId"); v != "" {
		q = q.Where("s.template_id = ?", v)
	}
	if v := c.Query("versionId"); v != "" {
		q = q.Where("s.version_id = ?", v)
	}
	var items []submissionListItem
	if err := q.Order("s.business_date desc,s.id desc").Find(&items).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询填报失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (s *Server) updateSubmissionRow(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, scopeErr := scope.WritableDepartmentID(s.db)
	if scopeErr != nil {
		response.Fail(c, 400, "department_required", scopeErr.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var row model.SubmissionRow
	if err := s.db.First(&row, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "明细不存在", nil)
		return
	}
	var sub model.Submission
	if err := s.db.First(&sub, row.SubmissionID).Error; err != nil {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	if sub.DepartmentID != departmentID {
		response.Fail(c, 404, "not_found", "明细不存在", nil)
		return
	}
	uid := c.GetUint64("user_id")
	if !s.canEditRow(uid, &sub, &row) {
		response.Fail(c, 403, "forbidden", "只能修改自己的明细或权限范围内明细", nil)
		return
	}
	var req struct {
		Values []valueInput `json:"values"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	before := []model.FieldValue{}
	_ = s.db.Where("submission_id = ? AND row_id = ?", sub.ID, row.ID).Find(&before).Error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("submission_id = ? AND row_id = ?", sub.ID, row.ID).Delete(&model.FieldValue{}).Error; err != nil {
			return err
		}
		for _, val := range req.Values {
			if err := tx.Create(&model.FieldValue{SubmissionID: sub.ID, RowID: &row.ID, FieldKey: val.FieldKey, ValueType: val.ValueType, ValueText: val.ValueText, ValueNumber: val.ValueNumber, RepeatIndex: val.RepeatIndex, Source: defaultSource(val.Source), CreatedBy: uid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, 500, "save_failed", "保存明细失败", err.Error())
		return
	}
	s.audit(c, "submission_row", row.ID, "update", before, req.Values)
	response.OK(c, gin.H{"id": row.ID})
}

func (s *Server) voidSubmissionRow(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, scopeErr := scope.WritableDepartmentID(s.db)
	if scopeErr != nil {
		response.Fail(c, 400, "department_required", scopeErr.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var row model.SubmissionRow
	if err := s.db.First(&row, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "明细不存在", nil)
		return
	}
	var sub model.Submission
	if err := s.db.First(&sub, row.SubmissionID).Error; err != nil {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	if sub.DepartmentID != departmentID {
		response.Fail(c, 404, "not_found", "明细不存在", nil)
		return
	}
	uid := c.GetUint64("user_id")
	if !s.canVoidRow(uid, &sub, &row) {
		response.Fail(c, 403, "forbidden", "没有作废该明细的权限", nil)
		return
	}
	if row.Status == model.SubmissionStatusVoided {
		response.Fail(c, 400, "nothing_to_void", "该明细已废除", nil)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	reason, err := normalizeActionReason(req.Reason, "作废")
	if err != nil {
		response.Fail(c, 400, "reason_required", err.Error(), nil)
		return
	}
	before := row
	now := time.Now()
	row.Status = model.SubmissionStatusVoided
	row.VoidReason = reason
	row.VoidedBy = &uid
	row.VoidedAt = &now
	row.RestoreReason = ""
	row.RestoredBy = nil
	row.RestoredAt = nil
	if err := s.db.Save(&row).Error; err != nil {
		response.Fail(c, 500, "void_failed", "作废失败", err.Error())
		return
	}
	s.audit(c, "submission_row", row.ID, "void", before, row)
	response.OK(c, row)
}

func (s *Server) restoreSubmissionRow(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, scopeErr := scope.WritableDepartmentID(s.db)
	if scopeErr != nil {
		response.Fail(c, 400, "department_required", scopeErr.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var row model.SubmissionRow
	if err := s.db.First(&row, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "明细不存在", nil)
		return
	}
	var sub model.Submission
	if err := s.db.First(&sub, row.SubmissionID).Error; err != nil {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	if sub.DepartmentID != departmentID {
		response.Fail(c, 404, "not_found", "明细不存在", nil)
		return
	}
	uid := c.GetUint64("user_id")
	if !s.canEditRow(uid, &sub, &row) {
		response.Fail(c, 403, "forbidden", "没有恢复该明细的权限", nil)
		return
	}
	if row.Status == model.SubmissionStatusActive {
		response.Fail(c, 400, "nothing_to_restore", "该明细当前有效", nil)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	reason, err := normalizeActionReason(req.Reason, "恢复")
	if err != nil {
		response.Fail(c, 400, "reason_required", err.Error(), nil)
		return
	}
	before := row
	now := time.Now()
	row.Status = model.SubmissionStatusActive
	row.RestoreReason = reason
	row.RestoredBy = &uid
	row.RestoredAt = &now
	row.VoidReason = ""
	row.VoidedBy = nil
	row.VoidedAt = nil
	if err := s.db.Save(&row).Error; err != nil {
		response.Fail(c, 500, "restore_failed", "恢复失败", err.Error())
		return
	}
	s.audit(c, "submission_row", row.ID, "restore", before, row)
	response.OK(c, row)
}

func (s *Server) voidSubmission(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, scopeErr := scope.WritableDepartmentID(s.db)
	if scopeErr != nil {
		response.Fail(c, 400, "department_required", scopeErr.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	if sub.DepartmentID != departmentID {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	uid := c.GetUint64("user_id")
	if !s.canVoidSubmission(uid, &sub) {
		response.Fail(c, 403, "forbidden", "没有废除该填报的权限", nil)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	reason, err := normalizeActionReason(req.Reason, "废除")
	if err != nil {
		response.Fail(c, 400, "reason_required", err.Error(), nil)
		return
	}
	now := time.Now()
	res := s.db.Model(&model.SubmissionRow{}).
		Where("submission_id = ? AND status = ?", sub.ID, model.SubmissionStatusActive).
		Updates(map[string]any{
			"status":         model.SubmissionStatusVoided,
			"void_reason":    reason,
			"voided_by":      uid,
			"voided_at":      now,
			"restore_reason": "",
			"restored_by":    nil,
			"restored_at":    nil,
		})
	if res.Error != nil {
		response.Fail(c, 500, "void_failed", "废除失败", res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		response.Fail(c, 400, "nothing_to_void", "该填报已废除，没有可废除的明细", nil)
		return
	}
	s.audit(c, "submission", sub.ID, "void", nil, gin.H{"reason": reason, "rows": res.RowsAffected})
	response.OK(c, gin.H{"id": sub.ID, "voidedRows": res.RowsAffected})
}

func (s *Server) restoreSubmission(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, scopeErr := scope.WritableDepartmentID(s.db)
	if scopeErr != nil {
		response.Fail(c, 400, "department_required", scopeErr.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	if sub.DepartmentID != departmentID {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	uid := c.GetUint64("user_id")
	if !s.canRestoreSubmission(uid, &sub) {
		response.Fail(c, 403, "forbidden", "没有恢复该填报的权限", nil)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	reason, err := normalizeActionReason(req.Reason, "恢复")
	if err != nil {
		response.Fail(c, 400, "reason_required", err.Error(), nil)
		return
	}
	now := time.Now()
	res := s.db.Model(&model.SubmissionRow{}).
		Where("submission_id = ? AND status = ?", sub.ID, model.SubmissionStatusVoided).
		Updates(map[string]any{
			"status":         model.SubmissionStatusActive,
			"restore_reason": reason,
			"restored_by":    uid,
			"restored_at":    now,
			"void_reason":    "",
			"voided_by":      nil,
			"voided_at":      nil,
		})
	if res.Error != nil {
		response.Fail(c, 500, "restore_failed", "恢复失败", res.Error.Error())
		return
	}
	if res.RowsAffected == 0 {
		response.Fail(c, 400, "nothing_to_restore", "该填报没有可恢复的明细", nil)
		return
	}
	s.audit(c, "submission", sub.ID, "restore", nil, gin.H{"reason": reason, "rows": res.RowsAffected})
	response.OK(c, gin.H{"id": sub.ID, "restoredRows": res.RowsAffected})
}

func (s *Server) canVoidSubmission(uid uint64, sub *model.Submission) bool {
	if self, _ := s.authorizer.Can(uid, rbac.PermSubmissionVoidOwn, nil); self && sub.CreatedBy == uid {
		return true
	}
	return s.canLine(uid, rbac.PermSubmissionVoidScope, sub.LineID)
}

func (s *Server) canRestoreSubmission(uid uint64, sub *model.Submission) bool {
	if self, _ := s.authorizer.Can(uid, rbac.PermSubmissionEditOwn, nil); self && sub.CreatedBy == uid {
		return true
	}
	return s.canLine(uid, rbac.PermSubmissionEditScope, sub.LineID)
}

func (s *Server) canEditRow(uid uint64, sub *model.Submission, row *model.SubmissionRow) bool {
	if self, _ := s.authorizer.Can(uid, rbac.PermSubmissionEditOwn, nil); self && row.OperatorID == uid {
		return true
	}
	return s.canLine(uid, rbac.PermSubmissionEditScope, sub.LineID)
}
func (s *Server) canVoidRow(uid uint64, sub *model.Submission, row *model.SubmissionRow) bool {
	if self, _ := s.authorizer.Can(uid, rbac.PermSubmissionVoidOwn, nil); self && row.OperatorID == uid {
		return true
	}
	return s.canLine(uid, rbac.PermSubmissionVoidScope, sub.LineID)
}
func defaultSource(v string) string {
	if v == "" {
		return "manual"
	}
	return v
}

var _ = json.Marshal
var _ = fmt.Sprintf

type submissionListItem struct {
	ID                uint64    `json:"id"`
	TemplateID        uint64    `json:"templateId"`
	VersionID         uint64    `json:"versionId"`
	LineID            uint64    `json:"lineId"`
	BusinessDate      time.Time `json:"businessDate"`
	ShiftPlanItemID   uint64    `json:"shiftPlanItemId"`
	ShiftNameSnapshot string    `json:"shiftNameSnapshot"`
	RepeatReason      string    `json:"repeatReason"`
	CreatedBy         uint64    `json:"createdBy"`
	SubmittedAt       time.Time `json:"submittedAt"`
}

type currentValueResponse struct {
	FieldKey    string   `json:"fieldKey"`
	ValueType   string   `json:"valueType"`
	ValueText   string   `json:"valueText"`
	ValueNumber *float64 `json:"valueNumber"`
	RepeatIndex int      `json:"repeatIndex"`
	Source      string   `json:"source"`
}

type currentRowResponse struct {
	RowID      uint64                 `json:"rowId"`
	RowNo      int                    `json:"rowNo"`
	OperatorID uint64                 `json:"operatorId"`
	OperatedAt time.Time              `json:"operatedAt"`
	Values     []currentValueResponse `json:"values"`
}

type mySubmissionListItem struct {
	ID                uint64    `json:"id" gorm:"column:id"`
	TemplateID        uint64    `json:"templateId" gorm:"column:template_id"`
	TemplateName      string    `json:"templateName" gorm:"column:template_name"`
	Kind              string    `json:"kind" gorm:"column:kind"`
	VersionID         uint64    `json:"versionId" gorm:"column:version_id"`
	VersionNo         int       `json:"versionNo" gorm:"column:version_no"`
	LineID            uint64    `json:"lineId" gorm:"column:line_id"`
	LineName          string    `json:"lineName" gorm:"column:line_name"`
	BusinessDate      time.Time `json:"businessDate" gorm:"column:business_date"`
	ShiftPlanItemID   uint64    `json:"shiftPlanItemId" gorm:"column:shift_plan_item_id"`
	ShiftNameSnapshot string    `json:"shiftNameSnapshot" gorm:"column:shift_name_snapshot"`
	RepeatReason      string    `json:"repeatReason" gorm:"column:repeat_reason"`
	Source            string    `json:"source" gorm:"column:source"`
	SubmittedAt       time.Time `json:"submittedAt" gorm:"column:submitted_at"`
	OwnRowCount       int       `json:"ownRowCount" gorm:"-"`
	ActiveRowCount    int       `json:"activeRowCount" gorm:"-"`
	VoidedRowCount    int       `json:"voidedRowCount" gorm:"-"`
	Superseded        bool      `json:"superseded" gorm:"-"`
}

type mySubmissionValueResponse struct {
	FieldKey    string   `json:"fieldKey"`
	FieldLabel  string   `json:"fieldLabel"`
	ValueType   string   `json:"valueType"`
	ValueText   string   `json:"valueText"`
	ValueNumber *float64 `json:"valueNumber"`
	OutOfRange  bool     `json:"outOfRange"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Unit        string   `json:"unit"`
	RepeatIndex int      `json:"repeatIndex"`
	Source      string   `json:"source"`
}

type mySubmissionRowResponse struct {
	ID            uint64                      `json:"id"`
	RowNo         int                         `json:"rowNo"`
	Status        string                      `json:"status"`
	Superseded    bool                        `json:"superseded"`
	OperatedAt    time.Time                   `json:"operatedAt"`
	VoidReason    string                      `json:"voidReason"`
	VoidedAt      *time.Time                  `json:"voidedAt"`
	RestoreReason string                      `json:"restoreReason"`
	RestoredAt    *time.Time                  `json:"restoredAt"`
	Values        []mySubmissionValueResponse `json:"values"`
}

type mySubmissionDetailResponse struct {
	mySubmissionListItem
	Rows []mySubmissionRowResponse `json:"rows"`
}

func (s *Server) listMySubmissions(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	uid := c.GetUint64("user_id")
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	var lineIDs []uint64
	var err error
	if lineID > 0 {
		if !s.canLineInScope(scope, rbac.PermExecutionView, lineID) {
			response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
			return
		}
		lineIDs, err = s.expandLineIDs([]uint64{lineID})
	} else {
		lineIDs, err = s.authorizer.AllowedLines(uid, scope.DepartmentID)
	}
	if err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	if len(lineIDs) == 0 {
		response.OK(c, gin.H{"items": []mySubmissionListItem{}, "total": 0, "page": 1, "size": 20})
		return
	}

	ownRows := s.db.Table("submission_rows r").
		Select("r.submission_id").
		Where("r.operator_id = ? AND r.deleted_at IS NULL", uid)
	base := s.db.Table("submissions s").
		Joins("JOIN form_templates t ON t.id = s.template_id").
		Joins("JOIN form_versions v ON v.id = s.version_id").
		Joins("JOIN lines l ON l.id = s.line_id").
		Where("s.deleted_at IS NULL AND s.line_id IN ?", lineIDs).
		Where("s.id IN (?)", ownRows)

	page, size := pageParams(c)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询历史填报失败", err.Error())
		return
	}
	var items []mySubmissionListItem
	if err := base.Select("s.id,s.template_id,t.name AS template_name,t.kind,s.version_id,v.version_no,s.line_id,l.name AS line_name,s.business_date,s.shift_plan_item_id,s.shift_name_snapshot,s.repeat_reason,s.source,s.submitted_at").
		Order("s.business_date DESC,s.id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Scan(&items).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询历史填报失败", err.Error())
		return
	}
	s.attachMySubmissionCounts(uid, items)
	latest := s.latestInspectionIDs(lineIDs)
	for i := range items {
		item := &items[i]
		if item.ActiveRowCount > 0 && (item.Kind == model.FormKindFirstInspection || item.Kind == model.FormKindFinalInspection) {
			key := inspectionKey(item.LineID, item.TemplateID, item.BusinessDate.Format("2006-01-02"), item.ShiftPlanItemID)
			item.Superseded = latest[key] != item.ID
		}
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "size": size})
}

func (s *Server) attachMySubmissionCounts(uid uint64, items []mySubmissionListItem) {
	if len(items) == 0 {
		return
	}
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	var rows []model.SubmissionRow
	_ = s.db.Where("submission_id IN ? AND operator_id = ? AND deleted_at IS NULL", ids, uid).Find(&rows).Error
	type counts struct {
		own    int
		active int
		voided int
	}
	byID := make(map[uint64]counts, len(items))
	for _, row := range rows {
		value := byID[row.SubmissionID]
		value.own++
		if row.Status == model.SubmissionStatusActive {
			value.active++
		} else if row.Status == model.SubmissionStatusVoided {
			value.voided++
		}
		byID[row.SubmissionID] = value
	}
	for i := range items {
		value := byID[items[i].ID]
		items[i].OwnRowCount = value.own
		items[i].ActiveRowCount = value.active
		items[i].VoidedRowCount = value.voided
	}
}

func (s *Server) mySubmissionDetail(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	uid := c.GetUint64("user_id")
	var item mySubmissionListItem
	err := s.db.Table("submissions s").
		Select("s.id,s.template_id,t.name AS template_name,t.kind,s.version_id,v.version_no,s.line_id,l.name AS line_name,s.business_date,s.shift_plan_item_id,s.shift_name_snapshot,s.repeat_reason,s.source,s.submitted_at").
		Joins("JOIN form_templates t ON t.id = s.template_id").
		Joins("JOIN form_versions v ON v.id = s.version_id").
		Joins("JOIN lines l ON l.id = s.line_id").
		Where("s.id = ? AND s.deleted_at IS NULL", id).
		Scan(&item).Error
	if err != nil || item.ID == 0 {
		response.Fail(c, 404, "not_found", "填报不存在", nil)
		return
	}
	if !s.canLineInScope(scope, rbac.PermExecutionView, item.LineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	var rows []model.SubmissionRow
	if err := s.db.Where("submission_id = ? AND operator_id = ? AND deleted_at IS NULL", item.ID, uid).Order("row_no").Find(&rows).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询历史明细失败", err.Error())
		return
	}
	if len(rows) == 0 {
		response.Fail(c, 404, "not_found", "历史明细不存在", nil)
		return
	}
	item.OwnRowCount = len(rows)
	for _, row := range rows {
		if row.Status == model.SubmissionStatusActive {
			item.ActiveRowCount++
		} else if row.Status == model.SubmissionStatusVoided {
			item.VoidedRowCount++
		}
	}

	rowIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		rowIDs = append(rowIDs, row.ID)
	}
	var values []model.FieldValue
	if err := s.db.Where("row_id IN ?", rowIDs).Order("row_id,repeat_index,id").Find(&values).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询历史数据失败", err.Error())
		return
	}
	fieldByKey := map[string]service.FieldDefinition{}
	var version model.FormVersion
	if err := s.db.First(&version, item.VersionID).Error; err == nil {
		if schema, parseErr := service.ParseSchema([]byte(version.Schema)); parseErr == nil {
			fieldByKey = service.FieldMap(schema)
		}
	}
	valuesByRow := make(map[uint64][]mySubmissionValueResponse)
	for _, value := range values {
		if value.RowID == nil {
			continue
		}
		field := fieldByKey[value.FieldKey]
		label := field.Label
		if label == "" {
			label = value.FieldKey
		}
		valuesByRow[*value.RowID] = append(valuesByRow[*value.RowID], mySubmissionValueResponse{
			FieldKey: value.FieldKey, FieldLabel: label, ValueType: value.ValueType,
			ValueText: displayFieldText(value.ValueText, field), ValueNumber: value.ValueNumber,
			OutOfRange: isValueOutOfRange(value.ValueNumber, field), Min: field.Min, Max: field.Max,
			Unit:        field.Unit,
			RepeatIndex: value.RepeatIndex, Source: value.Source,
		})
	}
	latest := s.latestInspectionIDs([]uint64{item.LineID})
	item.Superseded = false
	if item.ActiveRowCount > 0 && (item.Kind == model.FormKindFirstInspection || item.Kind == model.FormKindFinalInspection) {
		key := inspectionKey(item.LineID, item.TemplateID, item.BusinessDate.Format("2006-01-02"), item.ShiftPlanItemID)
		item.Superseded = latest[key] != item.ID
	}
	result := mySubmissionDetailResponse{mySubmissionListItem: item, Rows: make([]mySubmissionRowResponse, 0, len(rows))}
	for _, row := range rows {
		result.Rows = append(result.Rows, mySubmissionRowResponse{
			ID: row.ID, RowNo: row.RowNo, Status: row.Status, OperatedAt: row.OperatedAt,
			VoidReason: row.VoidReason, VoidedAt: row.VoidedAt,
			RestoreReason: row.RestoreReason, RestoredAt: row.RestoredAt,
			Superseded: item.Superseded && row.Status == model.SubmissionStatusActive,
			Values:     valuesByRow[row.ID],
		})
	}
	response.OK(c, result)
}

func normalizeActionReason(raw, action string) (string, error) {
	reason := strings.TrimSpace(raw)
	if reason == "" {
		return "", fmt.Errorf("%s必须填写原因", action)
	}
	if utf8.RuneCountInString(reason) > 500 {
		return "", fmt.Errorf("%s原因不能超过500字", action)
	}
	return reason, nil
}

func (s *Server) currentSubmission(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	versionID, _ := strconv.ParseUint(c.Query("versionId"), 10, 64)
	shiftID, _ := strconv.ParseUint(c.Query("shiftPlanItemId"), 10, 64)
	businessDate := strings.TrimSpace(c.Query("businessDate"))
	if lineID == 0 || versionID == 0 || shiftID == 0 || businessDate == "" {
		response.OK(c, gin.H{"data": nil})
		return
	}
	if !s.canLineInScope(scope, rbac.PermExecutionView, lineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	var submission model.Submission
	err := s.db.Where("line_id = ? AND version_id = ? AND shift_plan_item_id = ? AND business_date = ? AND deleted_at IS NULL", lineID, versionID, shiftID, businessDate).Order("submitted_at desc,id desc").First(&submission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.OK(c, gin.H{"data": nil})
		return
	}
	if err != nil {
		response.Fail(c, 500, "db_error", "查询最近填报失败", err.Error())
		return
	}
	var rows []model.SubmissionRow
	if err := s.db.Where("submission_id = ? AND status = ?", submission.ID, model.SubmissionStatusActive).Order("row_no").Find(&rows).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询填报明细失败", err.Error())
		return
	}
	rowIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		rowIDs = append(rowIDs, row.ID)
	}
	var values []model.FieldValue
	if len(rowIDs) > 0 {
		if err := s.db.Where("row_id IN ?", rowIDs).Order("row_id,repeat_index,id").Find(&values).Error; err != nil {
			response.Fail(c, 500, "db_error", "查询填报数据失败", err.Error())
			return
		}
	}
	valuesByRow := map[uint64][]currentValueResponse{}
	for _, value := range values {
		if value.RowID == nil {
			continue
		}
		valuesByRow[*value.RowID] = append(valuesByRow[*value.RowID], currentValueResponse{FieldKey: value.FieldKey, ValueType: value.ValueType, ValueText: value.ValueText, ValueNumber: value.ValueNumber, RepeatIndex: value.RepeatIndex, Source: value.Source})
	}
	resultRows := make([]currentRowResponse, 0, len(rows))
	for _, row := range rows {
		resultRows = append(resultRows, currentRowResponse{RowID: row.ID, RowNo: row.RowNo, OperatorID: row.OperatorID, OperatedAt: row.OperatedAt, Values: valuesByRow[row.ID]})
	}
	response.OK(c, gin.H{"data": gin.H{"submissionId": submission.ID, "submittedAt": submission.SubmittedAt, "rows": resultRows}})
}
