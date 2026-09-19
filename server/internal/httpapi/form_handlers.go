package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type formTemplateRequest struct {
	Code   string          `json:"code"`
	Name   string          `json:"name"`
	Kind   string          `json:"kind"`
	Schema json.RawMessage `json:"schema"`
}

type formTemplateListItem struct {
	model.FormTemplate
	AssignmentCount int64 `json:"assignmentCount"`
	SubmissionCount int64 `json:"submissionCount"`
	IsUnused        bool  `json:"isUnused"`
	CanDelete       bool  `json:"canDelete"`
}

func (s *Server) listFormTemplates(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	page, size := pageParams(c)
	buildQuery := func() *gorm.DB {
		q := scope.Filter(s.db.Model(&model.FormTemplate{}), "department_id")
		if kind := c.Query("kind"); kind != "" {
			q = q.Where("kind = ?", kind)
		}
		return q
	}
	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询模板失败", err.Error())
		return
	}
	var templates []model.FormTemplate
	if err := buildQuery().Preload("Department").Order("id desc").Offset((page - 1) * size).Limit(size).Find(&templates).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询模板失败", err.Error())
		return
	}
	items := make([]formTemplateListItem, 0, len(templates))
	for _, tpl := range templates {
		assignmentCount, submissionCount, err := s.formTemplateUsage(tpl.ID)
		if err != nil {
			response.Fail(c, 500, "db_error", "检查模板使用情况失败", err.Error())
			return
		}
		items = append(items, formTemplateListItem{
			FormTemplate:    tpl,
			AssignmentCount: assignmentCount,
			SubmissionCount: submissionCount,
			IsUnused:        assignmentCount == 0 && submissionCount == 0,
			CanDelete:       true,
		})
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "size": size})
}

func (s *Server) createFormTemplate(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, err := scope.WritableDepartmentID(s.db)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	var req formTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	schema, err := parseRawSchema(req.Schema)
	if err != nil {
		response.Fail(c, 400, "validation_error", "表单结构错误", err.Error())
		return
	}
	tpl := model.FormTemplate{DepartmentID: departmentID, Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name), Kind: req.Kind, CreatedBy: c.GetUint64("user_id")}
	if tpl.Code == "" || tpl.Name == "" {
		response.Fail(c, 400, "validation_error", "编码和名称不能为空", nil)
		return
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&tpl).Error; err != nil {
			return err
		}
		schemaJSON, _ := json.Marshal(schema)
		return tx.Create(&model.FormVersion{TemplateID: tpl.ID, VersionNo: 1, Schema: datatypes.JSON(schemaJSON), Status: model.VersionDraft, CreatedBy: tpl.CreatedBy}).Error
	})
	if err != nil {
		response.Fail(c, 409, "create_failed", "模板创建失败", err.Error())
		return
	}
	s.audit(c, "form_template", tpl.ID, "create", nil, tpl)
	response.Created(c, tpl)
}

func (s *Server) getFormTemplate(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var tpl model.FormTemplate
	if err := scope.Filter(s.db.Preload("Department"), "department_id").First(&tpl, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "模板不存在", nil)
		return
	}
	var versions []model.FormVersion
	if err := s.db.Where("template_id = ?", id).Order("version_no desc").Find(&versions).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询版本失败", err.Error())
		return
	}
	var assignments []model.LineFormAssignment
	_ = s.db.Where("template_id = ?", id).Find(&assignments).Error
	response.OK(c, gin.H{"template": tpl, "versions": versions, "assignments": assignments})
}

func (s *Server) createFormVersion(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var tpl model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&tpl, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "模板不存在", nil)
		return
	}
	var req struct {
		Schema json.RawMessage `json:"schema"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	schema, err := parseRawSchema(req.Schema)
	if err != nil {
		response.Fail(c, 400, "validation_error", "表单结构错误", err.Error())
		return
	}
	var maxVersion int
	_ = s.db.Model(&model.FormVersion{}).Where("template_id = ?", id).Select("COALESCE(MAX(version_no),0)").Scan(&maxVersion).Error
	schemaJSON, _ := json.Marshal(schema)
	version := model.FormVersion{TemplateID: id, VersionNo: maxVersion + 1, Schema: datatypes.JSON(schemaJSON), Status: model.VersionDraft, CreatedBy: c.GetUint64("user_id")}
	if err := s.db.Create(&version).Error; err != nil {
		response.Fail(c, 500, "create_failed", "创建版本失败", err.Error())
		return
	}
	s.audit(c, "form_version", version.ID, "create", nil, version)
	response.Created(c, version)
}

func (s *Server) publishFormVersion(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var version model.FormVersion
	if err := s.db.First(&version, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "版本不存在", nil)
		return
	}
	var tpl model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&tpl, version.TemplateID).Error; err != nil {
		response.Fail(c, http.StatusNotFound, "not_found", "版本不存在", nil)
		return
	}
	var req struct {
		LineIDs []uint64 `json:"lineIds"`
	}
	_ = c.ShouldBindJSON(&req)
	if len(req.LineIDs) == 0 {
		if err := s.db.Model(&model.LineFormAssignment{}).Where("template_id = ?", version.TemplateID).Pluck("line_id", &req.LineIDs).Error; err != nil {
			response.Fail(c, 500, "db_error", "查询模板已分配产线失败", err.Error())
			return
		}
	}
	if err := s.ensureLinesInDepartment(req.LineIDs, tpl.DepartmentID); err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", "产线不属于当前部门", nil)
		return
	}
	now := time.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&version).Updates(map[string]any{"status": model.VersionPublished, "published_at": now}).Error; err != nil {
			return err
		}
		for _, lineID := range req.LineIDs {
			var assignment model.LineFormAssignment
			err := tx.Where("line_id = ? AND template_id = ?", lineID, version.TemplateID).First(&assignment).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&model.LineFormAssignment{LineID: lineID, TemplateID: version.TemplateID, ActiveVersionID: version.ID}).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				if err := tx.Model(&assignment).Update("active_version_id", version.ID).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, 500, "publish_failed", "发布失败", err.Error())
		return
	}
	s.audit(c, "form_version", version.ID, "publish", nil, gin.H{"lineIds": req.LineIDs})
	response.OK(c, gin.H{"version": version, "lineIds": req.LineIDs})
}

func (s *Server) rollbackFormVersion(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var version model.FormVersion
	if err := s.db.First(&version, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "版本不存在", nil)
		return
	}
	var tpl model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&tpl, version.TemplateID).Error; err != nil {
		response.Fail(c, http.StatusNotFound, "not_found", "版本不存在", nil)
		return
	}
	var req struct {
		LineIDs []uint64 `json:"lineIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.LineIDs) == 0 {
		response.Fail(c, 400, "validation_error", "回滚必须选择产线", nil)
		return
	}
	if err := s.ensureLinesInDepartment(req.LineIDs, tpl.DepartmentID); err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", "产线不属于当前部门", nil)
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, lineID := range req.LineIDs {
			res := tx.Model(&model.LineFormAssignment{}).Where("line_id = ? AND template_id = ?", lineID, version.TemplateID).Update("active_version_id", version.ID)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, 500, "rollback_failed", "回滚失败", err.Error())
		return
	}
	s.audit(c, "form_version", version.ID, "rollback", nil, gin.H{"lineIds": req.LineIDs})
	response.OK(c, gin.H{"versionId": version.ID, "lineIds": req.LineIDs})
}

func (s *Server) copyFormTemplate(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var source model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&source, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "模板不存在", nil)
		return
	}
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	if req.Kind == "" {
		req.Kind = model.FormKindFinalInspection
	}
	var version model.FormVersion
	if err := s.db.Where("template_id = ?", id).Order("version_no desc").First(&version).Error; err != nil {
		response.Fail(c, 400, "validation_error", "源模板没有版本", nil)
		return
	}
	target := model.FormTemplate{DepartmentID: source.DepartmentID, Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name), Kind: req.Kind, PairedTemplateID: &source.ID, CreatedBy: c.GetUint64("user_id")}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&target).Error; err != nil {
			return err
		}
		if err := tx.Model(&source).Update("paired_template_id", target.ID).Error; err != nil {
			return err
		}
		return tx.Create(&model.FormVersion{TemplateID: target.ID, VersionNo: 1, Schema: version.Schema, Status: model.VersionDraft, CreatedBy: target.CreatedBy}).Error
	})
	if err != nil {
		response.Fail(c, 500, "copy_failed", "复制模板失败", err.Error())
		return
	}
	response.Created(c, target)
}

func (s *Server) syncPairedTemplate(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var source model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&source, id).Error; err != nil || source.PairedTemplateID == nil {
		response.Fail(c, 400, "validation_error", "模板没有配对模板", nil)
		return
	}
	var paired model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&paired, *source.PairedTemplateID).Error; err != nil {
		response.Fail(c, 400, "validation_error", "配对模板不存在或不属于当前部门", nil)
		return
	}
	var sv model.FormVersion
	if err := s.db.Where("template_id = ?", id).Order("version_no desc").First(&sv).Error; err != nil {
		response.Fail(c, 400, "validation_error", "源模板没有版本", nil)
		return
	}
	var maxVersion int
	s.db.Model(&model.FormVersion{}).Where("template_id = ?", *source.PairedTemplateID).Select("COALESCE(MAX(version_no),0)").Scan(&maxVersion)
	version := model.FormVersion{TemplateID: *source.PairedTemplateID, VersionNo: maxVersion + 1, Schema: sv.Schema, Status: model.VersionDraft, CreatedBy: c.GetUint64("user_id")}
	if err := s.db.Create(&version).Error; err != nil {
		response.Fail(c, 500, "sync_failed", "同步失败", err.Error())
		return
	}
	response.Created(c, version)
}

func (s *Server) executionLines(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	ids, err := s.authorizer.AllowedLines(c.GetUint64("user_id"), scope.DepartmentID)
	if err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	var lines []model.Line
	if len(ids) == 0 {
		response.OK(c, gin.H{"items": []model.Line{}})
		return
	}
	if err := s.db.Where("id IN ? AND enabled = ?", ids, true).Order("level, code, id").Find(&lines).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	sortLinesByDepartmentAndCode(lines)
	response.OK(c, gin.H{"items": lines})
}

func (s *Server) executionForms(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	if lineID == 0 || !s.canLineInScope(scope, rbac.PermExecutionView, lineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	q := s.db.Table("line_form_assignments a").Select("t.id AS template_id,t.code,t.name,t.kind,v.id AS version_id,v.version_no,v.schema").
		Joins("JOIN form_templates t ON t.id=a.template_id").Joins("JOIN form_versions v ON v.id=a.active_version_id").
		Where("a.line_id = ? AND t.deleted_at IS NULL", lineID)
	if kind := c.Query("kind"); kind != "" {
		q = q.Where("t.kind = ?", kind)
	}
	var rows []executionFormItem
	if err := q.Order("t.id").Scan(&rows).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询任务失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows})
}

func (s *Server) executionFormDetail(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	versionID, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var version model.FormVersion
	if err := s.db.First(&version, versionID).Error; err != nil {
		response.Fail(c, 404, "not_found", "表单版本不存在", nil)
		return
	}
	var tpl model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&tpl, version.TemplateID).Error; err != nil {
		response.Fail(c, 404, "not_found", "表单不存在", nil)
		return
	}
	response.OK(c, gin.H{"template": tpl, "version": version})
}

func parseRawSchema(raw json.RawMessage) (service.FormSchema, error) {
	if len(raw) == 0 {
		return service.FormSchema{}, errors.New("schema不能为空")
	}
	return service.ParseSchema(raw)
}

func (s *Server) executionShifts(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if scope.AllDepartments {
		response.Fail(c, http.StatusBadRequest, "department_required", "请先选择部门", nil)
		return
	}
	plan, err := service.ActiveShiftPlan(s.db, scope.DepartmentID)
	if err != nil {
		response.Fail(c, 500, "shift_unavailable", "未配置可用班次", err.Error())
		return
	}
	current, _ := service.CalculateShift(*plan, time.Now())
	var currentItem *model.ShiftPlanItem
	if current != nil {
		currentItem = &current.Item
	}
	response.OK(c, gin.H{"plan": plan, "items": plan.Items, "current": currentItem})
}

func (s *Server) repeatCheck(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	shiftID, _ := strconv.ParseUint(c.Query("shiftPlanItemId"), 10, 64)
	if lineID == 0 || shiftID == 0 || c.Query("businessDate") == "" {
		response.Fail(c, 400, "validation_error", "缺少查询参数", nil)
		return
	}
	if !s.canLineInScope(scope, rbac.PermExecutionView, lineID) {
		response.Fail(c, http.StatusForbidden, "forbidden", "没有该产线权限", nil)
		return
	}
	var shiftItem model.ShiftPlanItem
	if err := s.db.First(&shiftItem, shiftID).Error; err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", "班次不存在", nil)
		return
	}
	var plan model.ShiftPlan
	if err := s.db.First(&plan, shiftItem.PlanID).Error; err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", "班次不存在", nil)
		return
	}
	var line model.Line
	if err := s.db.First(&line, lineID).Error; err != nil || line.DepartmentID != plan.DepartmentID {
		response.Fail(c, http.StatusBadRequest, "validation_error", "产线和班次不属于同一部门", nil)
		return
	}
	var count int64
	err := s.db.Model(&model.Submission{}).Where("line_id = ? AND shift_plan_item_id = ? AND business_date = ?", lineID, shiftID, c.Query("businessDate")).Count(&count).Error
	if err != nil {
		response.Fail(c, 500, "db_error", "查询失败", err.Error())
		return
	}
	response.OK(c, gin.H{"required": count > 0, "count": count})
}

type executionFormItem struct {
	TemplateID uint64         `json:"templateId"`
	Code       string         `json:"code"`
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	VersionID  uint64         `json:"versionId"`
	VersionNo  int            `json:"versionNo"`
	Schema     datatypes.JSON `json:"schema"`
}

type assignmentItemRequest struct {
	LineID    uint64 `json:"lineId"`
	VersionID uint64 `json:"versionId"`
}

func (s *Server) listTemplateAssignments(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var template model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&template, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "模板不存在", nil)
		return
	}
	var lines []model.Line
	if err := s.db.Where("department_id = ? AND enabled = ?", template.DepartmentID, true).Order("level, code, id").Find(&lines).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	sortLinesByCode(lines)
	var assignments []model.LineFormAssignment
	_ = s.db.Where("template_id = ?", id).Find(&assignments).Error
	assignmentByLine := map[uint64]model.LineFormAssignment{}
	for _, assignment := range assignments {
		assignmentByLine[assignment.LineID] = assignment
	}
	versionIDs := make([]uint64, 0, len(assignments))
	for _, assignment := range assignments {
		versionIDs = append(versionIDs, assignment.ActiveVersionID)
	}
	versionMap := map[uint64]model.FormVersion{}
	if len(versionIDs) > 0 {
		var versions []model.FormVersion
		_ = s.db.Where("id IN ?", versionIDs).Find(&versions).Error
		for _, version := range versions {
			versionMap[version.ID] = version
		}
	}
	result := make([]gin.H, 0, len(lines))
	for _, line := range lines {
		assignment := assignmentByLine[line.ID]
		version := versionMap[assignment.ActiveVersionID]
		result = append(result, gin.H{"lineId": line.ID, "lineName": line.Name, "versionId": assignment.ActiveVersionID, "versionNo": version.VersionNo, "assigned": assignment.ID != 0})
	}
	response.OK(c, gin.H{"items": result})
}

func (s *Server) updateTemplateAssignments(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var template model.FormTemplate
	if err := scope.Filter(s.db, "department_id").First(&template, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "模板不存在", nil)
		return
	}
	var req struct {
		Items []assignmentItemRequest `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	versionIDSet := map[uint64]bool{}
	for _, item := range req.Items {
		if item.LineID == 0 || item.VersionID == 0 {
			response.Fail(c, 400, "validation_error", "产线和版本不能为空", nil)
			return
		}
		versionIDSet[item.VersionID] = true
	}
	versionIDs := make([]uint64, 0, len(versionIDSet))
	for versionID := range versionIDSet {
		versionIDs = append(versionIDs, versionID)
	}
	if len(versionIDs) > 0 {
		var count int64
		err := s.db.Model(&model.FormVersion{}).Where("id IN ? AND template_id = ? AND status = ?", versionIDs, id, model.VersionPublished).Count(&count).Error
		if err != nil || int(count) != len(versionIDs) {
			response.Fail(c, 400, "validation_error", "只能分配给已发布版本", nil)
			return
		}
	}
	inputLineIDs := make([]uint64, 0, len(req.Items))
	for _, item := range req.Items {
		inputLineIDs = append(inputLineIDs, item.LineID)
	}
	if err := s.ensureLinesInDepartment(inputLineIDs, template.DepartmentID); err != nil {
		response.Fail(c, http.StatusBadRequest, "validation_error", "产线不属于当前部门", nil)
		return
	}
	expanded, err := s.expandAssignmentItems(req.Items)
	if err != nil {
		response.Fail(c, 500, "db_error", "查询子产线失败", err.Error())
		return
	}
	req.Items = expanded
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if len(req.Items) == 0 {
			return tx.Where("template_id = ?", id).Delete(&model.LineFormAssignment{}).Error
		}
		lineIDs := make([]uint64, 0, len(req.Items))
		for _, item := range req.Items {
			lineIDs = append(lineIDs, item.LineID)
		}
		if err := tx.Where("template_id = ? AND line_id NOT IN ?", id, lineIDs).Delete(&model.LineFormAssignment{}).Error; err != nil {
			return err
		}
		for _, item := range req.Items {
			var assignment model.LineFormAssignment
			err := tx.Where("line_id = ? AND template_id = ?", item.LineID, id).First(&assignment).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&model.LineFormAssignment{LineID: item.LineID, TemplateID: id, ActiveVersionID: item.VersionID}).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else if err := tx.Model(&assignment).Update("active_version_id", item.VersionID).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, 500, "assignment_failed", "保存产线分配失败", err.Error())
		return
	}
	s.audit(c, "form_template", id, "assign_lines", nil, req.Items)
	response.OK(c, gin.H{"items": req.Items})
}

// expandAssignmentItems 展开父产线分配：父产线选定的版本会同步到其全部子产线，
// 子产线上显式提交的版本优先于父产线的级联值。
func (s *Server) expandAssignmentItems(items []assignmentItemRequest) ([]assignmentItemRequest, error) {
	explicit := make(map[uint64]uint64, len(items))
	all := make(map[uint64]uint64, len(items))
	for _, item := range items {
		explicit[item.LineID] = item.VersionID
		all[item.LineID] = item.VersionID
	}
	for _, item := range items {
		related, err := s.expandLineIDs([]uint64{item.LineID})
		if err != nil {
			return nil, err
		}
		for _, lineID := range related {
			if _, ok := explicit[lineID]; ok {
				continue
			}
			all[lineID] = item.VersionID
		}
	}
	lineIDs := make([]uint64, 0, len(all))
	for lineID := range all {
		lineIDs = append(lineIDs, lineID)
	}
	sort.Slice(lineIDs, func(i, j int) bool { return lineIDs[i] < lineIDs[j] })
	out := make([]assignmentItemRequest, 0, len(lineIDs))
	for _, lineID := range lineIDs {
		out = append(out, assignmentItemRequest{LineID: lineID, VersionID: all[lineID]})
	}
	return out, nil
}
