// 文件说明：日志数据 Excel 导出处理器，负责筛选、分组和生成工作簿。

package httpapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type exportRequest struct {
	LineID      uint64   `json:"lineId"`
	DateFrom    string   `json:"dateFrom"`
	DateTo      string   `json:"dateTo"`
	TemplateIDs []uint64 `json:"templateIds"`
	Filler      string   `json:"filler"`
	LineIDs     []uint64 `json:"-"`
}

type exportValue struct {
	FieldKey        string
	Label           string
	Unit            string
	ValueType       string
	ValueText       string
	ValueNumber     *float64
	RepeatIndex     int
	Source          string
	RowNo           int
	OperatorName    string
	OperatorAccount string
	OperatedAt      time.Time
}

type exportSubmission struct {
	ID                uint64
	TemplateID        uint64
	VersionID         uint64
	LineID            uint64
	ShiftPlanItemID   uint64
	LineName          string
	BusinessDate      string
	ShiftNameSnapshot string
	SubmittedAt       time.Time
	TemplateName      string
	VersionNo         int
	Kind              string
	OperatorName      string
	OperatorAccount   string
	Values            []exportValue
}

// exportData 校验权限和日期范围后导出授权产线及其子产线的数据。
func (s *Server) exportData(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	var req exportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	if req.LineID == 0 || !s.canLineInScope(scope, rbac.PermLogView, req.LineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	if req.DateFrom == "" || req.DateTo == "" {
		response.Fail(c, 400, "validation_error", "必须选择日期范围", nil)
		return
	}
	from, err1 := time.ParseInLocation("2006-01-02", req.DateFrom, time.Local)
	to, err2 := time.ParseInLocation("2006-01-02", req.DateTo, time.Local)
	if err1 != nil || err2 != nil || to.Sub(from) > 31*24*time.Hour {
		response.Fail(c, 400, "validation_error", "日期范围无效或超过一个月", nil)
		return
	}
	lineIDs, err := s.expandLineIDs([]uint64{req.LineID})
	if err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	req.LineIDs = lineIDs
	s.exportWorkbook(c, req)
}

// exportWorkbook 导出与日志列表相同筛选范围内的全部记录：首末检按业务日期分成工作表，
// 报表与登记表合并到"数据"工作表，所有内容都放在同一个 Excel 文件中。
func (s *Server) exportWorkbook(c *gin.Context, req exportRequest) {
	subs := s.loadExportSubmissions(req, nil)
	inspection := make([]exportSubmission, 0, len(subs))
	forms := make([]exportSubmission, 0, len(subs))
	for _, sub := range subs {
		switch sub.Kind {
		case model.FormKindFirstInspection, model.FormKindFinalInspection:
			inspection = append(inspection, sub)
		case model.FormKindReport, model.FormKindRegistration:
			forms = append(forms, sub)
		}
	}
	if len(inspection) == 0 && len(forms) == 0 {
		response.Fail(c, 404, "no_data", "没有找到可导出的记录", nil)
		return
	}
	inspection = latestInspectionSubmissions(inspection)

	f := excelize.NewFile()
	if len(inspection) > 0 {
		writeInspectionSheets(f, inspection)
	}
	if len(forms) > 0 {
		writeFormsSheet(f, forms)
	}
	// excelize 在仅剩一个工作表时不会执行删除，需先建好数据表再移除默认的 Sheet1
	_ = f.DeleteSheet("Sheet1")
	if sheets := f.GetSheetList(); len(sheets) > 0 {
		if index, err := f.GetSheetIndex(sheets[len(sheets)-1]); err == nil {
			f.SetActiveSheet(index)
		}
	}
	writeWorkbook(c, f, exportFileName(len(inspection) > 0, len(forms) > 0, req.DateFrom, req.DateTo))
}

// writeInspectionSheets 把首检/末检记录按业务日期写入各自的工作表。
func writeInspectionSheets(f *excelize.File, subs []exportSubmission) {
	sheetRows := map[string]int{}
	for _, sub := range subs {
		sheet := "首检_" + strings.ReplaceAll(sub.BusinessDate, "-", "")
		if sub.Kind == model.FormKindFinalInspection {
			sheet = "末检_" + strings.ReplaceAll(sub.BusinessDate, "-", "")
		}
		if _, ok := sheetRows[sheet]; !ok {
			if _, err := f.NewSheet(sheet); err != nil {
				continue
			}
			sheetRows[sheet] = 1
			_ = f.SetSheetRow(sheet, "A1", &[]any{"日期", "班次", "产线", "项目", "测量值", "单位", "填报人", "工号", "提交时间"})
		}
		for _, value := range sub.Values {
			line := sheetRows[sheet] + 1
			sheetRows[sheet] = line
			_ = f.SetSheetRow(sheet, fmt.Sprintf("A%d", line), &[]any{sub.BusinessDate, sub.ShiftNameSnapshot, sub.LineName, value.Label, displayExportValue(value), value.Unit, value.OperatorName, value.OperatorAccount, value.OperatedAt.Format("2006-01-02 15:04:05")})
		}
	}
}

// writeFormsSheet 把报表与登记表合并到同一张"数据"工作表：一行一条填报明细、一列一个填写项目。
func writeFormsSheet(f *excelize.File, subs []exportSubmission) {
	columns := make([]string, 0, 16)
	columnIndex := map[string]int{}
	for _, sub := range subs {
		for _, value := range sub.Values {
			label := exportFieldLabel(value)
			if _, ok := columnIndex[label]; ok {
				continue
			}
			columnIndex[label] = len(columns)
			columns = append(columns, label)
		}
	}

	sheet := "数据"
	_, _ = f.NewSheet(sheet)
	header := []any{"业务日期", "班次", "产线", "表单", "版本", "明细", "填报人", "工号", "提交时间"}
	for _, label := range columns {
		header = append(header, label)
	}
	_ = f.SetSheetRow(sheet, "A1", &header)

	line := 1
	for _, sub := range subs {
		valuesByRow := map[int][]exportValue{}
		rowNos := make([]int, 0, 4)
		for _, value := range sub.Values {
			if _, ok := valuesByRow[value.RowNo]; !ok {
				rowNos = append(rowNos, value.RowNo)
			}
			valuesByRow[value.RowNo] = append(valuesByRow[value.RowNo], value)
		}
		sort.Ints(rowNos)
		for _, rowNo := range rowNos {
			cells := make([]any, len(columns))
			operatorName, operatorAccount := "", ""
			for _, value := range valuesByRow[rowNo] {
				if operatorName == "" {
					operatorName, operatorAccount = value.OperatorName, value.OperatorAccount
				}
				idx, ok := columnIndex[exportFieldLabel(value)]
				if !ok {
					continue
				}
				cells[idx] = appendExportCell(cells[idx], displayExportValue(value))
			}
			line++
			record := []any{sub.BusinessDate, sub.ShiftNameSnapshot, sub.LineName, sub.TemplateName, sub.VersionNo, rowNo, operatorName, operatorAccount, sub.SubmittedAt.Format("2006-01-02 15:04:05")}
			record = append(record, cells...)
			_ = f.SetSheetRow(sheet, fmt.Sprintf("A%d", line), &record)
		}
	}
}

// latestInspectionSubmissions 同一产线、同一模板、同一业务日期与班次的首末检只保留最新一条：
// 重复首检会覆盖此前填报，历史版本不再进入导出结果。
func latestInspectionSubmissions(subs []exportSubmission) []exportSubmission {
	if len(subs) == 0 {
		return subs
	}
	latest := map[string]uint64{}
	for _, sub := range subs {
		key := inspectionKey(sub.LineID, sub.TemplateID, sub.BusinessDate, sub.ShiftPlanItemID)
		if sub.ID > latest[key] {
			latest[key] = sub.ID
		}
	}
	out := make([]exportSubmission, 0, len(subs))
	for _, sub := range subs {
		if latest[inspectionKey(sub.LineID, sub.TemplateID, sub.BusinessDate, sub.ShiftPlanItemID)] == sub.ID {
			out = append(out, sub)
		}
	}
	return out
}

// inspectionKey 用于识别"同一产线 + 模板 + 业务日期 + 班次"的首末检记录。
func inspectionKey(lineID uint64, templateID uint64, businessDate string, shiftPlanItemID uint64) string {
	return fmt.Sprintf("%d|%d|%s|%d", lineID, templateID, businessDate, shiftPlanItemID)
}

func exportFileName(hasInspection bool, hasForms bool, dateFrom string, dateTo string) string {
	switch {
	case hasInspection && hasForms:
		return fmt.Sprintf("首末检与报表登记表_%s_%s.xlsx", dateFrom, dateTo)
	case hasForms:
		return fmt.Sprintf("报表登记表_%s_%s.xlsx", dateFrom, dateTo)
	default:
		return fmt.Sprintf("首末检_%s_%s.xlsx", dateFrom, dateTo)
	}
}

// exportFieldLabel 返回导出时使用的项目列名；未配置显示名称时退回字段标识。
func exportFieldLabel(value exportValue) string {
	if label := strings.TrimSpace(value.Label); label != "" {
		return label
	}
	return value.FieldKey
}

// appendExportCell 把同一明细同一项目的多个值合并进一个单元格。
func appendExportCell(current any, next any) any {
	if next == nil || next == "" {
		return current
	}
	if current == nil || current == "" {
		return next
	}
	return fmt.Sprintf("%v、%v", current, next)
}

// loadExportSubmissions 一次性加载版本、用户、明细和字段值，保证导出历史可读。
func (s *Server) loadExportSubmissions(req exportRequest, kinds []string) []exportSubmission {
	var submissions []model.Submission
	q := s.db.Where("line_id IN ? AND deleted_at IS NULL AND business_date BETWEEN ? AND ?", req.LineIDs, req.DateFrom, req.DateTo)
	if len(req.TemplateIDs) > 0 {
		q = q.Where("template_id IN ?", req.TemplateIDs)
	}
	if filler := strings.TrimSpace(req.Filler); filler != "" {
		var userIDs []uint64
		_ = s.db.Model(&model.User{}).Where("name ILIKE ? OR account ILIKE ?", "%"+filler+"%", "%"+filler+"%").Pluck("id", &userIDs).Error
		q = q.Where("created_by IN ?", userIDs)
	}
	if err := q.Order("business_date,id").Find(&submissions).Error; err != nil {
		return nil
	}
	if len(submissions) == 0 {
		return nil
	}

	templateIDs := make([]uint64, 0, len(submissions))
	versionIDs := make([]uint64, 0, len(submissions))
	userIDs := make([]uint64, 0, len(submissions))
	submissionIDs := make([]uint64, 0, len(submissions))
	for _, sub := range submissions {
		templateIDs = append(templateIDs, sub.TemplateID)
		versionIDs = append(versionIDs, sub.VersionID)
		userIDs = append(userIDs, sub.CreatedBy)
		submissionIDs = append(submissionIDs, sub.ID)
	}
	templates := map[uint64]model.FormTemplate{}
	versions := map[uint64]model.FormVersion{}
	users := map[uint64]model.User{}
	var templateRows []model.FormTemplate
	_ = s.db.Unscoped().Where("id IN ?", templateIDs).Find(&templateRows).Error
	for _, row := range templateRows {
		templates[row.ID] = row
	}
	var versionRows []model.FormVersion
	_ = s.db.Where("id IN ?", versionIDs).Find(&versionRows).Error
	for _, row := range versionRows {
		versions[row.ID] = row
	}
	var userRows []model.User
	_ = s.db.Unscoped().Where("id IN ?", userIDs).Find(&userRows).Error
	for _, row := range userRows {
		users[row.ID] = row
	}
	var lines []model.Line
	_ = s.db.Unscoped().Where("id IN ?", req.LineIDs).Find(&lines).Error
	lineNameByID := map[uint64]string{}
	for _, line := range lines {
		lineNameByID[line.ID] = line.Name
	}

	var submissionRows []model.SubmissionRow
	_ = s.db.Where("submission_id IN ? AND status = ?", submissionIDs, model.SubmissionStatusActive).Order("submission_id,row_no").Find(&submissionRows).Error
	rowByID := map[uint64]model.SubmissionRow{}
	operatorIDs := make([]uint64, 0, len(submissionRows))
	for _, row := range submissionRows {
		rowByID[row.ID] = row
		operatorIDs = append(operatorIDs, row.OperatorID)
	}
	var operatorRows []model.User
	_ = s.db.Unscoped().Where("id IN ?", operatorIDs).Find(&operatorRows).Error
	operatorByID := map[uint64]model.User{}
	for _, row := range operatorRows {
		operatorByID[row.ID] = row
	}

	var values []model.FieldValue
	_ = s.db.Where("submission_id IN ?", submissionIDs).Order("submission_id,row_id,repeat_index,id").Find(&values).Error
	valuesBySubmission := map[uint64][]model.FieldValue{}
	for _, value := range values {
		valuesBySubmission[value.SubmissionID] = append(valuesBySubmission[value.SubmissionID], value)
	}

	kindSet := map[string]bool{}
	for _, kind := range kinds {
		kindSet[kind] = true
	}
	fieldMaps := map[uint64]map[string]service.FieldDefinition{}
	result := make([]exportSubmission, 0, len(submissions))
	for _, sub := range submissions {
		template := templates[sub.TemplateID]
		if len(kindSet) > 0 && !kindSet[template.Kind] {
			continue
		}
		version := versions[sub.VersionID]
		if _, ok := fieldMaps[sub.VersionID]; !ok {
			schema, err := service.ParseSchema([]byte(version.Schema))
			if err == nil {
				fieldMaps[sub.VersionID] = service.FieldMap(schema)
			} else {
				fieldMaps[sub.VersionID] = map[string]service.FieldDefinition{}
			}
		}
		item := exportSubmission{ID: sub.ID, TemplateID: sub.TemplateID, VersionID: sub.VersionID, LineID: sub.LineID, ShiftPlanItemID: sub.ShiftPlanItemID, LineName: lineNameByID[sub.LineID], BusinessDate: sub.BusinessDate.Format("2006-01-02"), ShiftNameSnapshot: sub.ShiftNameSnapshot, SubmittedAt: sub.SubmittedAt, TemplateName: template.Name, VersionNo: version.VersionNo, Kind: template.Kind, OperatorName: users[sub.CreatedBy].Name, OperatorAccount: users[sub.CreatedBy].Account}
		for _, value := range valuesBySubmission[sub.ID] {
			if value.RowID == nil {
				continue
			}
			row, ok := rowByID[*value.RowID]
			if !ok {
				continue
			}
			field := fieldMaps[sub.VersionID][value.FieldKey]
			label := field.Label
			if label == "" {
				label = value.FieldKey
			}
			operator := operatorByID[row.OperatorID]
			item.Values = append(item.Values, exportValue{FieldKey: value.FieldKey, Label: label, Unit: field.Unit, ValueType: value.ValueType, ValueText: displayFieldText(value.ValueText, field), ValueNumber: value.ValueNumber, RepeatIndex: value.RepeatIndex, Source: value.Source, RowNo: row.RowNo, OperatorName: operator.Name, OperatorAccount: operator.Account, OperatedAt: row.OperatedAt})
		}
		result = append(result, item)
	}
	return result
}

func displayExportValue(value exportValue) any {
	if value.ValueNumber != nil {
		return *value.ValueNumber
	}
	return value.ValueText
}
func writeWorkbook(c *gin.Context, f *excelize.File, name string) {
	buffer, err := f.WriteToBuffer()
	if err != nil {
		response.Fail(c, 500, "export_failed", "生成Excel失败", err.Error())
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}
