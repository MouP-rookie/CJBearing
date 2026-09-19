// 文件说明：填报日志、日历、审计日志、备份记录和仪表报文查询接口。

package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service"
	"github.com/gin-gonic/gin"
)

type logDetailItem struct {
	RowID           uint64     `json:"rowId"`
	RowNo           int        `json:"rowNo"`
	FieldKey        string     `json:"fieldKey"`
	FieldLabel      string     `json:"fieldLabel"`
	ValueType       string     `json:"valueType"`
	ValueText       string     `json:"valueText"`
	ValueNumber     *float64   `json:"valueNumber"`
	OutOfRange      bool       `json:"outOfRange"`
	Min             *float64   `json:"min,omitempty"`
	Max             *float64   `json:"max,omitempty"`
	Unit            string     `json:"unit"`
	RepeatIndex     int        `json:"repeatIndex"`
	Source          string     `json:"source"`
	OperatorID      uint64     `json:"operatorId"`
	OperatorName    string     `json:"operatorName"`
	OperatorAccount string     `json:"operatorAccount"`
	OperatedAt      time.Time  `json:"operatedAt"`
	Status          string     `json:"status"`
	VoidReason      string     `json:"voidReason"`
	VoidedAt        *time.Time `json:"voidedAt"`
	RestoreReason   string     `json:"restoreReason"`
	RestoredAt      *time.Time `json:"restoredAt"`
}

type logListItem struct {
	ID                uint64          `json:"id"`
	TemplateID        uint64          `json:"templateId"`
	VersionID         uint64          `json:"versionId"`
	VersionNo         int             `json:"versionNo"`
	LineID            uint64          `json:"lineId"`
	LineName          string          `json:"lineName"`
	BusinessDate      time.Time       `json:"businessDate"`
	ShiftPlanItemID   uint64          `json:"shiftPlanItemId"`
	ShiftNameSnapshot string          `json:"shiftNameSnapshot"`
	RepeatReason      string          `json:"repeatReason"`
	Source            string          `json:"source"`
	CreatedBy         uint64          `json:"createdBy"`
	FillerName        string          `json:"fillerName"`
	FillerAccount     string          `json:"fillerAccount"`
	SubmittedAt       time.Time       `json:"submittedAt"`
	TemplateName      string          `json:"templateName"`
	Kind              string          `json:"kind"`
	Status            string          `json:"status"`
	VoidReason        string          `json:"voidReason"`
	Superseded        bool            `json:"superseded"`
	Details           []logDetailItem `gorm:"-" json:"details"`
}

type calendarItem struct {
	BusinessDate time.Time `json:"businessDate"`
	Count        int64     `json:"count"`
}

// listLogs 按授权产线和筛选条件返回主填报，并补充状态、字段详情和上下限。
func (s *Server) listLogs(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	if lineID == 0 || !s.canLineInScope(scope, rbac.PermLogView, lineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	lineIDs, err := s.expandLineIDs([]uint64{lineID})
	if err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	q := s.db.Table("submissions s").
		Select("s.id,s.template_id,s.version_id,v.version_no,s.line_id,l.name AS line_name,s.business_date,s.shift_plan_item_id,s.shift_name_snapshot,s.repeat_reason,s.source,s.created_by,u.name AS filler_name,u.account AS filler_account,s.submitted_at,t.name AS template_name,t.kind").
		Joins("JOIN form_templates t ON t.id=s.template_id").
		Joins("JOIN form_versions v ON v.id=s.version_id").
		Joins("JOIN lines l ON l.id=s.line_id").
		Joins("JOIN users u ON u.id=s.created_by").
		Where("s.line_id IN ? AND s.deleted_at IS NULL", lineIDs)
	if v := c.Query("templateId"); v != "" {
		q = q.Where("s.template_id = ?", v)
	}
	if v := c.Query("versionId"); v != "" {
		q = q.Where("s.version_id = ?", v)
	}
	if v := c.Query("date"); v != "" {
		q = q.Where("s.business_date = ?", v)
	}
	if v := c.Query("dateFrom"); v != "" {
		q = q.Where("s.business_date >= ?", v)
	}
	if v := c.Query("dateTo"); v != "" {
		q = q.Where("s.business_date <= ?", v)
	}
	if v := c.Query("filler"); v != "" {
		q = q.Where("u.name ILIKE ? OR u.account ILIKE ?", "%"+v+"%", "%"+v+"%")
	}
	page, size := pageParams(c)
	var total int64
	_ = q.Count(&total).Error
	var items []logListItem
	if err := q.Order("s.business_date desc,s.id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询日志失败", err.Error())
		return
	}
	s.attachLogStates(lineIDs, items)
	s.attachLogDetails(items)
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "size": size})
}

// attachLogDetails 批量加载字段值并解析提交版本 Schema，避免逐条查询。
func (s *Server) attachLogDetails(items []logListItem) {
	if len(items) == 0 {
		return
	}
	submissionIDs := make([]uint64, 0, len(items))
	versionIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		submissionIDs = append(submissionIDs, item.ID)
		versionIDs = append(versionIDs, item.VersionID)
	}
	var versions []model.FormVersion
	_ = s.db.Where("id IN ?", versionIDs).Find(&versions).Error
	versionByID := map[uint64]model.FormVersion{}
	fieldByVersion := map[uint64]map[string]service.FieldDefinition{}
	for _, version := range versions {
		versionByID[version.ID] = version
		schema, err := service.ParseSchema([]byte(version.Schema))
		if err == nil {
			fieldByVersion[version.ID] = service.FieldMap(schema)
		}
	}
	var rows []model.SubmissionRow
	_ = s.db.Where("submission_id IN ?", submissionIDs).Order("submission_id,row_no").Find(&rows).Error
	rowByID := map[uint64]model.SubmissionRow{}
	operatorIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		rowByID[row.ID] = row
		operatorIDs = append(operatorIDs, row.OperatorID)
	}
	var operators []model.User
	_ = s.db.Where("id IN ?", operatorIDs).Find(&operators).Error
	operatorByID := map[uint64]model.User{}
	for _, operator := range operators {
		operatorByID[operator.ID] = operator
	}
	var values []model.FieldValue
	_ = s.db.Where("submission_id IN ?", submissionIDs).Order("submission_id,row_id,repeat_index,id").Find(&values).Error
	valuesBySubmission := map[uint64][]model.FieldValue{}
	for _, value := range values {
		valuesBySubmission[value.SubmissionID] = append(valuesBySubmission[value.SubmissionID], value)
	}

	for i := range items {
		item := &items[i]
		fields := fieldByVersion[item.VersionID]
		for _, value := range valuesBySubmission[item.ID] {
			if value.RowID == nil {
				continue
			}
			row, ok := rowByID[*value.RowID]
			if !ok {
				continue
			}
			field := fields[value.FieldKey]
			label := field.Label
			if label == "" {
				label = value.FieldKey
			}
			operator := operatorByID[row.OperatorID]
			item.Details = append(item.Details, logDetailItem{RowID: row.ID, RowNo: row.RowNo, FieldKey: value.FieldKey, FieldLabel: label, ValueType: value.ValueType, ValueText: displayFieldText(value.ValueText, field), ValueNumber: value.ValueNumber, OutOfRange: isValueOutOfRange(value.ValueNumber, field), Min: field.Min, Max: field.Max, Unit: field.Unit, RepeatIndex: value.RepeatIndex, Source: value.Source, OperatorID: row.OperatorID, OperatorName: operator.Name, OperatorAccount: operator.Account, OperatedAt: row.OperatedAt, Status: row.Status, VoidReason: row.VoidReason, VoidedAt: row.VoidedAt, RestoreReason: row.RestoreReason, RestoredAt: row.RestoredAt})
		}
	}
}

// attachLogStates 补充填报状态：没有任何有效明细的记为已废除；
// 首末检中不是该班次最新一条有效填报的，标记为已被覆盖（导出时不会重复导出）。
func (s *Server) attachLogStates(lineIDs []uint64, items []logListItem) {
	if len(items) == 0 {
		return
	}
	submissionIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		submissionIDs = append(submissionIDs, item.ID)
	}
	var rows []model.SubmissionRow
	_ = s.db.Where("submission_id IN ? AND deleted_at IS NULL", submissionIDs).Find(&rows).Error
	activeCount := map[uint64]int{}
	voidReason := map[uint64]string{}
	for _, row := range rows {
		if row.Status == model.SubmissionStatusActive {
			activeCount[row.SubmissionID]++
			continue
		}
		if row.VoidReason != "" && voidReason[row.SubmissionID] == "" {
			voidReason[row.SubmissionID] = row.VoidReason
		}
	}
	latest := s.latestInspectionIDs(lineIDs)
	for i := range items {
		item := &items[i]
		if activeCount[item.ID] == 0 {
			item.Status = model.SubmissionStatusVoided
			item.VoidReason = voidReason[item.ID]
		} else {
			item.Status = model.SubmissionStatusActive
		}
		if item.Kind == model.FormKindFirstInspection || item.Kind == model.FormKindFinalInspection {
			key := inspectionKey(item.LineID, item.TemplateID, item.BusinessDate.Format("2006-01-02"), item.ShiftPlanItemID)
			if item.Status == model.SubmissionStatusActive && latest[key] != item.ID {
				item.Superseded = true
			}
		}
	}
}

// latestInspectionIDs 返回指定产线下，每条(模板,业务日期,班次)中最新一条有效首末检填报的 ID。
func (s *Server) latestInspectionIDs(lineIDs []uint64) map[string]uint64 {
	out := map[string]uint64{}
	if len(lineIDs) == 0 {
		return out
	}
	type latestRow struct {
		ID              uint64
		LineID          uint64
		TemplateID      uint64
		BusinessDate    time.Time
		ShiftPlanItemID uint64
	}
	var rows []latestRow
	_ = s.db.Table("submissions s").
		Select("s.id, s.line_id, s.template_id, s.business_date, s.shift_plan_item_id").
		Joins("JOIN form_templates t ON t.id = s.template_id").
		Where("s.deleted_at IS NULL AND s.line_id IN ? AND t.kind IN ?", lineIDs, []string{model.FormKindFirstInspection, model.FormKindFinalInspection}).
		Where("EXISTS (SELECT 1 FROM submission_rows r WHERE r.submission_id = s.id AND r.status = ? AND r.deleted_at IS NULL)", model.SubmissionStatusActive).
		Scan(&rows).Error
	for _, row := range rows {
		key := inspectionKey(row.LineID, row.TemplateID, row.BusinessDate.Format("2006-01-02"), row.ShiftPlanItemID)
		if row.ID > out[key] {
			out[key] = row.ID
		}
	}
	return out
}

// logCalendar 按业务日期统计有有效明细的填报数量。
func (s *Server) logCalendar(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	if lineID == 0 || !s.canLineInScope(scope, rbac.PermLogView, lineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	lineIDs, err := s.expandLineIDs([]uint64{lineID})
	if err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	q := s.db.Table("submissions s").
		Select("s.business_date AS business_date, COUNT(*) AS count").
		Joins("JOIN users u ON u.id = s.created_by").
		Where("s.line_id IN ? AND s.deleted_at IS NULL", lineIDs).
		Where("EXISTS (SELECT 1 FROM submission_rows r WHERE r.submission_id = s.id AND r.status = ? AND r.deleted_at IS NULL)", model.SubmissionStatusActive)
	if v := c.Query("templateId"); v != "" {
		q = q.Where("s.template_id = ?", v)
	}
	if v := c.Query("dateFrom"); v != "" {
		q = q.Where("s.business_date >= ?", v)
	}
	if v := c.Query("dateTo"); v != "" {
		q = q.Where("s.business_date <= ?", v)
	}
	if v := c.Query("filler"); v != "" {
		q = q.Where("u.name ILIKE ? OR u.account ILIKE ?", "%"+v+"%", "%"+v+"%")
	}
	var items []calendarItem
	if err := q.Group("s.business_date").Order("s.business_date").Find(&items).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询日历失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

// listAuditLogs 在部门范围内检索审计记录。
func (s *Server) listAuditLogs(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	q := scope.FilterOptional(s.db.Table("audit_logs"), "department_id").Order("created_at desc")
	if entity := c.Query("entityType"); entity != "" {
		q = q.Where("entity_type = ?", entity)
	}
	if id := c.Query("entityId"); id != "" {
		q = q.Where("entity_id = ?", id)
	}
	page, size := pageParams(c)
	var total int64
	_ = q.Count(&total).Error
	var items []model.AuditLog
	if err := q.Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询审计日志失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "size": size})
}

// listBackups 按部门范围分页查询备份记录。
func (s *Server) listBackups(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	page, size := pageParams(c)
	items, total, err := s.backups.ListPage(scope.DepartmentID, scope.AllDepartments, page, size)
	if err != nil {
		response.Fail(c, 500, "db_error", "查询备份失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "size": size})
}

// createBackup 根据范围选择完整备份或部门归档，并再次校验超级管理员权限。
func (s *Server) createBackup(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	var req struct {
		Type string `json:"type"`
	}
	_ = c.ShouldBindJSON(&req)
	kind := strings.ToLower(strings.TrimSpace(req.Type))
	if kind == "" {
		if scope.AllDepartments {
			kind = model.BackupKindFull
		} else {
			kind = model.BackupKindDepartment
		}
	}
	var rec *model.BackupRecord
	var err error
	switch kind {
	case model.BackupKindFull:
		if scope.User.Role.Code != "super_admin" {
			response.Fail(c, http.StatusForbidden, "forbidden", "仅超级管理员可以生成完整数据库备份", nil)
			return
		}
		rec, err = s.backups.Create(s.cfg.DatabaseURL, c.GetUint64("user_id"))
	case model.BackupKindDepartment:
		departmentID, scopeErr := scope.WritableDepartmentID(s.db)
		if scopeErr != nil {
			response.Fail(c, http.StatusBadRequest, "department_required", scopeErr.Error(), nil)
			return
		}
		rec, err = s.backups.CreateDepartmentArchive(departmentID, c.GetUint64("user_id"))
	default:
		response.Fail(c, http.StatusBadRequest, "validation_error", "备份类型无效", nil)
		return
	}
	if err != nil {
		response.Fail(c, 500, "backup_failed", "备份失败", err.Error())
		return
	}
	response.Created(c, rec)
}

// downloadBackup 校验备份记录属于当前范围后返回附件。
func (s *Server) downloadBackup(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	rec, err := s.backups.GetForScope(id, scope.DepartmentID, scope.AllDepartments)
	if err != nil {
		response.Fail(c, 404, "not_found", "备份不存在", nil)
		return
	}
	c.FileAttachment(rec.FilePath, rec.FileName)
}

// listInstrumentMessages 返回部门范围内仪表原始报文，供后续调试和解析。
func (s *Server) listInstrumentMessages(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	page, size := pageParams(c)
	var total int64
	q := scope.FilterOptional(s.db.Table("instrument_messages"), "department_id")
	_ = q.Count(&total).Error
	var items []model.InstrumentMessage
	if err := q.Order("received_at desc").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询仪表报文失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "size": size, "serverTime": time.Now()})
}
