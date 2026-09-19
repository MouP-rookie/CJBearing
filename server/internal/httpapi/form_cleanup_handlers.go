package httpapi

import (
	"strings"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type formTemplateCleanupItem struct {
	ID              uint64 `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	AssignmentCount int64  `json:"assignmentCount"`
	SubmissionCount int64  `json:"submissionCount"`
}

type formTemplateCleanupSkipped struct {
	ID              uint64 `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	AssignmentCount int64  `json:"assignmentCount"`
	SubmissionCount int64  `json:"submissionCount"`
	Reason          string `json:"reason"`
}

func (s *Server) deleteUnusedFormTemplates(c *gin.Context) {
	var req struct {
		Kind        string   `json:"kind"`
		TemplateIDs []uint64 `json:"templateIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	if strings.TrimSpace(req.Kind) == "" && len(req.TemplateIDs) == 0 {
		response.Fail(c, 400, "validation_error", "必须指定要清理的模板类型或模板 ID", nil)
		return
	}
	q := s.db.Model(&model.FormTemplate{})
	if len(req.TemplateIDs) > 0 {
		q = q.Where("id IN ?", req.TemplateIDs)
	}
	if kind := strings.TrimSpace(req.Kind); kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var templates []model.FormTemplate
	if err := q.Order("id").Find(&templates).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询模板失败", err.Error())
		return
	}

	toDelete := make([]model.FormTemplate, 0, len(templates))
	deleted := make([]formTemplateCleanupItem, 0, len(templates))
	skipped := make([]formTemplateCleanupSkipped, 0)
	for _, tpl := range templates {
		assignmentCount, submissionCount, err := s.formTemplateUsage(tpl.ID)
		if err != nil {
			response.Fail(c, 500, "db_error", "检查模板使用情况失败", err.Error())
			return
		}
		if assignmentCount > 0 || submissionCount > 0 {
			reasons := make([]string, 0, 2)
			if assignmentCount > 0 {
				reasons = append(reasons, "已分配产线")
			}
			if submissionCount > 0 {
				reasons = append(reasons, "已有填报记录")
			}
			skipped = append(skipped, formTemplateCleanupSkipped{ID: tpl.ID, Code: tpl.Code, Name: tpl.Name, AssignmentCount: assignmentCount, SubmissionCount: submissionCount, Reason: strings.Join(reasons, "、")})
			continue
		}
		toDelete = append(toDelete, tpl)
		deleted = append(deleted, formTemplateCleanupItem{ID: tpl.ID, Code: tpl.Code, Name: tpl.Name, Kind: tpl.Kind})
	}

	if len(toDelete) > 0 {
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			for _, tpl := range toDelete {
				if err := tx.Model(&model.FormTemplate{}).Where("paired_template_id = ?", tpl.ID).Update("paired_template_id", nil).Error; err != nil {
					return err
				}
				if err := tx.Delete(&model.FormTemplate{}, tpl.ID).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			response.Fail(c, 500, "delete_failed", "删除未使用模板失败", err.Error())
			return
		}
		for _, tpl := range toDelete {
			s.audit(c, "form_template", tpl.ID, "delete_unused", nil, tpl)
		}
	}

	response.OK(c, gin.H{"deleted": len(deleted), "skipped": len(skipped), "items": deleted, "skippedItems": skipped})
}

func (s *Server) deleteFormTemplates(c *gin.Context) {
	var req struct {
		TemplateIDs []uint64 `json:"templateIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	if len(req.TemplateIDs) == 0 {
		response.Fail(c, 400, "validation_error", "请选择要删除的模板", nil)
		return
	}
	var templates []model.FormTemplate
	if err := s.db.Where("id IN ?", req.TemplateIDs).Order("id").Find(&templates).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询模板失败", err.Error())
		return
	}
	items := make([]formTemplateCleanupItem, 0, len(templates))
	for _, tpl := range templates {
		assignmentCount, submissionCount, err := s.formTemplateUsage(tpl.ID)
		if err != nil {
			response.Fail(c, 500, "db_error", "检查模板使用情况失败", err.Error())
			return
		}
		items = append(items, formTemplateCleanupItem{ID: tpl.ID, Code: tpl.Code, Name: tpl.Name, Kind: tpl.Kind, AssignmentCount: assignmentCount, SubmissionCount: submissionCount})
	}
	if len(templates) > 0 {
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			for _, tpl := range templates {
				if err := tx.Where("template_id = ?", tpl.ID).Delete(&model.LineFormAssignment{}).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.FormTemplate{}).Where("paired_template_id = ?", tpl.ID).Update("paired_template_id", nil).Error; err != nil {
					return err
				}
				if err := tx.Delete(&model.FormTemplate{}, tpl.ID).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			response.Fail(c, 500, "delete_failed", "删除模板失败", err.Error())
			return
		}
		for _, tpl := range templates {
			s.audit(c, "form_template", tpl.ID, "delete_with_history", nil, tpl)
		}
	}
	response.OK(c, gin.H{"deleted": len(items), "items": items})
}

func (s *Server) formTemplateUsage(templateID uint64) (int64, int64, error) {
	var assignmentCount int64
	if err := s.db.Table("line_form_assignments a").
		Joins("JOIN lines l ON l.id = a.line_id").
		Where("a.template_id = ? AND l.deleted_at IS NULL", templateID).
		Count(&assignmentCount).Error; err != nil {
		return 0, 0, err
	}
	var submissionCount int64
	if err := s.db.Unscoped().Model(&model.Submission{}).Where("template_id = ?", templateID).Count(&submissionCount).Error; err != nil {
		return 0, 0, err
	}
	return assignmentCount, submissionCount, nil
}
