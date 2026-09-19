package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	restoreArchiveVersion = 1
	maxRestoreJSONBytes   = 512 << 20
)

var errRestoreDryRun = errors.New("restore dry run completed")

type RestoreResult struct {
	DepartmentID   uint64 `json:"departmentId"`
	DepartmentCode string `json:"departmentCode"`
	DepartmentName string `json:"departmentName"`
	Users          int    `json:"users"`
	Lines          int    `json:"lines"`
	Products       int    `json:"products"`
	ShiftPlans     int    `json:"shiftPlans"`
	FormTemplates  int    `json:"formTemplates"`
	FormVersions   int    `json:"formVersions"`
	Submissions    int    `json:"submissions"`
	SubmissionRows int    `json:"submissionRows"`
	FieldValues    int    `json:"fieldValues"`
	AuditLogs      int    `json:"auditLogs"`
	InstrumentMsgs int    `json:"instrumentMessages"`
}

func (s *Service) RestoreDepartmentArchive(payload []byte, actorID uint64) (*RestoreResult, error) {
	_ = actorID
	data, err := parseAndValidateDepartmentArchive(payload)
	if err != nil {
		return nil, err
	}
	result := &RestoreResult{DepartmentCode: data.Department.Code, DepartmentName: data.Department.Name}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		return s.restoreDepartmentArchiveTx(tx, data, result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ValidateDepartmentArchive(payload []byte) (*RestoreResult, error) {
	data, err := parseAndValidateDepartmentArchive(payload)
	if err != nil {
		return nil, err
	}
	result := &RestoreResult{DepartmentCode: data.Department.Code, DepartmentName: data.Department.Name}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.restoreDepartmentArchiveTx(tx, data, result); err != nil {
			return err
		}
		return errRestoreDryRun
	})
	if errors.Is(err, errRestoreDryRun) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, errors.New("恢复校验未正常回滚")
}

func parseAndValidateDepartmentArchive(payload []byte) (*departmentArchive, error) {
	manifest, data, err := parseDepartmentArchive(payload)
	if err != nil {
		return nil, err
	}
	if manifest.Version != restoreArchiveVersion {
		return nil, fmt.Errorf("不支持的备份版本: %d", manifest.Version)
	}
	if strings.TrimSpace(data.Department.Code) == "" || strings.TrimSpace(data.Department.Name) == "" {
		return nil, errors.New("备份中缺少部门编码或名称")
	}
	return data, nil
}

func (s *Service) restoreDepartmentArchiveTx(tx *gorm.DB, data *departmentArchive, result *RestoreResult) error {
	deletedLines := idSet(data.DeletedLineIDs)
	deletedPlans := idSet(data.DeletedShiftPlanIDs)
	deletedTemplates := idSet(data.DeletedTemplateIDs)
	if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "restore_department:"+data.Department.Code).Error; err != nil {
		return err
	}

	var department model.Department
	err := tx.Where("code = ?", data.Department.Code).First(&department).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		department = model.Department{Code: data.Department.Code, Name: data.Department.Name, Enabled: data.Department.Enabled}
		if err := tx.Create(&department).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if err := tx.Model(&department).Updates(map[string]any{"name": data.Department.Name, "enabled": data.Department.Enabled}).Error; err != nil {
			return err
		}
	}
	result.DepartmentID = department.ID
	result.DepartmentName = data.Department.Name

	if err := clearDepartmentData(tx, department.ID); err != nil {
		return err
	}
	roleByID, err := loadArchiveRoles(tx, data.Users)
	if err != nil {
		return err
	}
	userMap, err := restoreUsers(tx, department.ID, data.Users, roleByID, result)
	if err != nil {
		return err
	}
	if err := fillMissingUsers(tx, department.ID, data, userMap, roleByID); err != nil {
		return err
	}
	lineMap, err := restoreLines(tx, department.ID, data.Lines, deletedLines, result)
	if err != nil {
		return err
	}
	if err := fillMissingLines(tx, department.ID, data, lineMap, result); err != nil {
		return err
	}
	if err := restoreProducts(tx, department.ID, data.Products, lineMap, result); err != nil {
		return err
	}
	_, shiftItemMap, err := restoreShifts(tx, department.ID, data.ShiftPlans, deletedPlans, result)
	if err != nil {
		return err
	}
	if err := fillMissingShiftItems(tx, department.ID, data, shiftItemMap, userMap, result); err != nil {
		return err
	}
	templateMap, versionMap, err := restoreForms(tx, department.ID, data.FormTemplates, data.FormVersions, data.LineFormAssignments, data.Submissions, lineMap, userMap, deletedTemplates, result)
	if err != nil {
		return err
	}
	_, _, err = restoreSubmissions(tx, department.ID, data.Submissions, data.SubmissionRows, data.FieldValues, templateMap, versionMap, lineMap, shiftItemMap, userMap, result)
	if err != nil {
		return err
	}
	if err := restoreAuditAndInstrument(tx, department.ID, data.AuditLogs, data.InstrumentMessages, userMap, result); err != nil {
		return err
	}
	if err := restoreDepartmentSettings(tx, department.ID, data.Settings, result); err != nil {
		return err
	}
	return nil
}

func fillMissingUsers(tx *gorm.DB, departmentID uint64, data *departmentArchive, userMap map[uint64]uint64, roles map[uint64]model.Role) error {
	required := map[uint64]bool{}
	for _, submission := range data.Submissions {
		required[submission.CreatedBy] = true
	}
	for _, row := range data.SubmissionRows {
		required[row.OperatorID] = true
		if row.VoidedBy != nil {
			required[*row.VoidedBy] = true
		}
		if row.RestoredBy != nil {
			required[*row.RestoredBy] = true
		}
	}
	for _, value := range data.FieldValues {
		required[value.CreatedBy] = true
	}
	roleID := uint64(0)
	for _, role := range roles {
		if role.Code != "super_admin" {
			roleID = role.ID
			break
		}
	}
	if len(required) > 0 && roleID == 0 {
		return errors.New("没有可用于恢复历史用户的普通角色")
	}
	hash, err := auth.HashPassword("restored-disabled-account")
	if err != nil {
		return err
	}
	for oldID := range required {
		if _, ok := userMap[oldID]; ok {
			continue
		}
		account := fmt.Sprintf("__restored_user_%d", oldID)
		var user model.User
		err := tx.Unscoped().Where("account = ?", account).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = model.User{
				Account: account, Name: fmt.Sprintf("历史用户%d", oldID), PasswordHash: hash,
				DepartmentID: &departmentID, RoleID: roleID, Enabled: false,
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		userMap[oldID] = user.ID
	}
	return nil
}

func fillMissingLines(tx *gorm.DB, departmentID uint64, data *departmentArchive, lineMap map[uint64]uint64, result *RestoreResult) error {
	required := map[uint64]bool{}
	for _, submission := range data.Submissions {
		required[submission.LineID] = true
	}
	for _, assignment := range data.LineFormAssignments {
		required[assignment.LineID] = true
	}
	for oldID := range required {
		if _, ok := lineMap[oldID]; ok {
			continue
		}
		line := model.Line{
			DepartmentID: departmentID, Code: fmt.Sprintf("__restored_line_%d", oldID),
			Name: fmt.Sprintf("历史产线%d", oldID), Level: 1, Enabled: false,
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		}
		if err := tx.Create(&line).Error; err != nil {
			return err
		}
		lineMap[oldID] = line.ID
		result.Lines++
	}
	return nil
}

func fillMissingShiftItems(tx *gorm.DB, departmentID uint64, data *departmentArchive, itemMap map[uint64]uint64, userMap map[uint64]uint64, result *RestoreResult) error {
	required := map[uint64]bool{}
	for _, submission := range data.Submissions {
		required[submission.ShiftPlanItemID] = true
	}
	if len(required) == 0 {
		return nil
	}
	createdBy := uint64(0)
	for _, userID := range userMap {
		createdBy = userID
		break
	}
	if createdBy == 0 {
		return errors.New("缺少可用于恢复历史班次的用户")
	}
	for oldID := range required {
		if _, ok := itemMap[oldID]; ok {
			continue
		}
		plan := model.ShiftPlan{
			DepartmentID: departmentID, Name: fmt.Sprintf("历史班次计划%d", oldID), EffectiveAt: time.Now(),
			CreatedBy: createdBy, DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		}
		if err := tx.Create(&plan).Error; err != nil {
			return err
		}
		item := model.ShiftPlanItem{PlanID: plan.ID, Name: fmt.Sprintf("历史班次%d", oldID), StartTime: "00:00", EndTime: "00:00"}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		itemMap[oldID] = item.ID
		result.ShiftPlans++
	}
	return nil
}

func parseDepartmentArchive(payload []byte) (*archiveManifest, *departmentArchive, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, nil, errors.New("备份文件不是有效的 ZIP 压缩包")
	}
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}
	manifestFile, ok := files["manifest.json"]
	if !ok {
		return nil, nil, errors.New("备份缺少 manifest.json")
	}
	dataFile, ok := files["data.json"]
	if !ok {
		return nil, nil, errors.New("备份缺少 data.json")
	}
	manifestBytes, err := readZipFile(manifestFile)
	if err != nil {
		return nil, nil, err
	}
	dataBytes, err := readZipFile(dataFile)
	if err != nil {
		return nil, nil, err
	}
	var manifest archiveManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, nil, fmt.Errorf("manifest.json 格式错误: %w", err)
	}
	var data departmentArchive
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, nil, fmt.Errorf("data.json 格式错误: %w", err)
	}
	return &manifest, &data, nil
}

func idSet(ids []uint64) map[uint64]bool {
	out := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func readZipFile(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > maxRestoreJSONBytes {
		return nil, fmt.Errorf("备份文件 %s 过大", file.Name)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, maxRestoreJSONBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxRestoreJSONBytes {
		return nil, fmt.Errorf("备份文件 %s 过大", file.Name)
	}
	return payload, nil
}

func clearDepartmentData(tx *gorm.DB, departmentID uint64) error {
	submissionIDs := tx.Unscoped().Model(&model.Submission{}).Select("id").Where("department_id = ?", departmentID)
	if err := tx.Unscoped().Where("submission_id IN (?)", submissionIDs).Delete(&model.FieldValue{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("submission_id IN (?)", submissionIDs).Delete(&model.SubmissionRow{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("department_id = ?", departmentID).Delete(&model.Submission{}).Error; err != nil {
		return err
	}

	templateIDs := tx.Unscoped().Model(&model.FormTemplate{}).Select("id").Where("department_id = ?", departmentID)
	lineIDs := tx.Unscoped().Model(&model.Line{}).Select("id").Where("department_id = ?", departmentID)
	if err := tx.Unscoped().Where("template_id IN (?) OR line_id IN (?)", templateIDs, lineIDs).Delete(&model.LineFormAssignment{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("template_id IN (?)", templateIDs).Delete(&model.FormVersion{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Model(&model.FormTemplate{}).Where("department_id = ?", departmentID).Update("paired_template_id", nil).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("department_id = ?", departmentID).Delete(&model.FormTemplate{}).Error; err != nil {
		return err
	}

	if err := tx.Exec("DELETE FROM product_lines WHERE product_id IN (SELECT id FROM products WHERE department_id = ?) OR line_id IN (SELECT id FROM lines WHERE department_id = ?)", departmentID, departmentID).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("department_id = ?", departmentID).Delete(&model.Product{}).Error; err != nil {
		return err
	}

	planIDs := tx.Unscoped().Model(&model.ShiftPlan{}).Select("id").Where("department_id = ?", departmentID)
	if err := tx.Unscoped().Where("plan_id IN (?)", planIDs).Delete(&model.ShiftPlanItem{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("department_id = ?", departmentID).Delete(&model.ShiftPlan{}).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("department_id = ?", departmentID).Delete(&model.Line{}).Error; err != nil {
		return err
	}
	if err := tx.Where("department_id = ?", departmentID).Delete(&model.AuditLog{}).Error; err != nil {
		return err
	}
	if err := tx.Where("department_id = ?", departmentID).Delete(&model.InstrumentMessage{}).Error; err != nil {
		return err
	}
	if err := tx.Where("department_id = ?", departmentID).Delete(&model.DepartmentSetting{}).Error; err != nil {
		return err
	}
	return nil
}

func loadArchiveRoles(tx *gorm.DB, users []archiveUser) (map[uint64]model.Role, error) {
	roleIDs := make([]uint64, 0, len(users))
	for _, user := range users {
		roleIDs = append(roleIDs, user.RoleID)
	}
	var roles []model.Role
	if len(roleIDs) > 0 {
		if err := tx.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
			return nil, err
		}
	}
	out := make(map[uint64]model.Role, len(roles))
	for _, role := range roles {
		if role.Code == "super_admin" {
			return nil, errors.New("部门备份中不能包含超级管理员角色")
		}
		out[role.ID] = role
	}
	for _, user := range users {
		if _, ok := out[user.RoleID]; !ok {
			return nil, fmt.Errorf("用户 %s 的角色不存在或已停用", user.Account)
		}
	}
	return out, nil
}

func restoreUsers(tx *gorm.DB, departmentID uint64, users []archiveUser, roles map[uint64]model.Role, result *RestoreResult) (map[uint64]uint64, error) {
	out := make(map[uint64]uint64, len(users))
	for _, archived := range users {
		if strings.TrimSpace(archived.Account) == "" || strings.TrimSpace(archived.PasswordHash) == "" {
			return nil, fmt.Errorf("备份用户 %d 缺少账号或密码哈希", archived.ID)
		}
		if _, ok := roles[archived.RoleID]; !ok {
			return nil, fmt.Errorf("用户 %s 的角色不存在", archived.Account)
		}
		var user model.User
		err := tx.Unscoped().Where("account = ?", archived.Account).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user = model.User{Account: archived.Account, Name: archived.Name, PasswordHash: archived.PasswordHash, DepartmentID: &departmentID, RoleID: archived.RoleID, Enabled: archived.Enabled, CreatedAt: archived.CreatedAt, UpdatedAt: archived.UpdatedAt}
			if archived.Deleted {
				user.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
			}
			if err := tx.Create(&user).Error; err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		} else {
			if user.DepartmentID == nil || *user.DepartmentID != departmentID {
				return nil, fmt.Errorf("账号 %s 已属于其他部门，无法恢复", archived.Account)
			}
			user.Name = archived.Name
			user.PasswordHash = archived.PasswordHash
			user.RoleID = archived.RoleID
			user.Enabled = archived.Enabled
			user.DepartmentID = &departmentID
			user.DeletedAt = gorm.DeletedAt{}
			if archived.Deleted {
				user.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
			}
			user.UpdatedAt = archived.UpdatedAt
			if err := tx.Unscoped().Save(&user).Error; err != nil {
				return nil, err
			}
		}
		out[archived.ID] = user.ID
		result.Users++
	}
	return out, nil
}

func restoreLines(tx *gorm.DB, departmentID uint64, lines []model.Line, deleted map[uint64]bool, result *RestoreResult) (map[uint64]uint64, error) {
	out := make(map[uint64]uint64, len(lines))
	for _, source := range lines {
		if source.Level != 1 {
			continue
		}
		line := source
		line.ID = 0
		line.DepartmentID = departmentID
		line.ParentID = nil
		if deleted[source.ID] {
			line.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		}
		if err := tx.Create(&line).Error; err != nil {
			return nil, err
		}
		out[source.ID] = line.ID
		result.Lines++
	}
	for _, source := range lines {
		if source.Level != 2 {
			continue
		}
		if source.ParentID == nil {
			return nil, fmt.Errorf("子产线 %s 缺少上级产线", source.Code)
		}
		parentID, ok := out[*source.ParentID]
		if !ok {
			return nil, fmt.Errorf("子产线 %s 的上级产线未包含在备份中", source.Code)
		}
		line := source
		line.ID = 0
		line.DepartmentID = departmentID
		line.ParentID = &parentID
		if deleted[source.ID] {
			line.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		}
		if err := tx.Create(&line).Error; err != nil {
			return nil, err
		}
		out[source.ID] = line.ID
		result.Lines++
	}
	return out, nil
}

func restoreProducts(tx *gorm.DB, departmentID uint64, products []model.Product, lineMap map[uint64]uint64, result *RestoreResult) error {
	for _, source := range products {
		lineIDs := make([]uint64, 0, len(source.Lines))
		for _, line := range source.Lines {
			if mapped, ok := lineMap[line.ID]; ok {
				lineIDs = append(lineIDs, mapped)
			}
		}
		product := source
		product.ID = 0
		product.DepartmentID = departmentID
		product.Lines = nil
		if err := tx.Create(&product).Error; err != nil {
			return err
		}
		if len(lineIDs) > 0 {
			var lines []model.Line
			if err := tx.Where("id IN ?", lineIDs).Find(&lines).Error; err != nil {
				return err
			}
			if err := tx.Model(&product).Association("Lines").Replace(lines); err != nil {
				return err
			}
		}
		result.Products++
	}
	return nil
}

func restoreShifts(tx *gorm.DB, departmentID uint64, plans []model.ShiftPlan, deleted map[uint64]bool, result *RestoreResult) (map[uint64]uint64, map[uint64]uint64, error) {
	planMap := make(map[uint64]uint64, len(plans))
	itemMap := map[uint64]uint64{}
	for _, source := range plans {
		plan := source
		plan.ID = 0
		plan.DepartmentID = departmentID
		plan.Items = nil
		if deleted[source.ID] {
			plan.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		}
		if err := tx.Create(&plan).Error; err != nil {
			return nil, nil, err
		}
		planMap[source.ID] = plan.ID
		for _, sourceItem := range source.Items {
			item := sourceItem
			item.ID = 0
			item.PlanID = plan.ID
			if err := tx.Create(&item).Error; err != nil {
				return nil, nil, err
			}
			itemMap[sourceItem.ID] = item.ID
		}
		result.ShiftPlans++
	}
	return planMap, itemMap, nil
}

func restoreForms(tx *gorm.DB, departmentID uint64, templates []model.FormTemplate, versions []model.FormVersion, assignments []model.LineFormAssignment, submissions []model.Submission, lineMap, userMap map[uint64]uint64, deletedTemplates map[uint64]bool, result *RestoreResult) (map[uint64]uint64, map[uint64]uint64, error) {
	templateMap := make(map[uint64]uint64, len(templates))
	for _, source := range templates {
		template := source
		template.ID = 0
		template.DepartmentID = departmentID
		template.PairedTemplateID = nil
		if deletedTemplates[source.ID] {
			template.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		}
		if err := tx.Create(&template).Error; err != nil {
			return nil, nil, err
		}
		templateMap[source.ID] = template.ID
		result.FormTemplates++
	}
	for _, source := range templates {
		if source.PairedTemplateID == nil {
			continue
		}
		mapped, ok := templateMap[*source.PairedTemplateID]
		if !ok {
			return nil, nil, fmt.Errorf("模板 %s 的配对模板未包含在备份中", source.Code)
		}
		if err := tx.Model(&model.FormTemplate{}).Where("id = ?", templateMap[source.ID]).Update("paired_template_id", mapped).Error; err != nil {
			return nil, nil, err
		}
	}
	versionMap := make(map[uint64]uint64, len(versions))
	for _, source := range versions {
		templateID, ok := templateMap[source.TemplateID]
		if !ok {
			return nil, nil, fmt.Errorf("版本 %d 的模板未包含在备份中", source.ID)
		}
		version := source
		version.ID = 0
		version.TemplateID = templateID
		if err := tx.Create(&version).Error; err != nil {
			return nil, nil, err
		}
		versionMap[source.ID] = version.ID
		result.FormVersions++
	}
	if err := fillMissingFormReferences(tx, departmentID, assignments, submissions, templateMap, versionMap, userMap, result); err != nil {
		return nil, nil, err
	}
	for _, source := range assignments {
		lineID, lineOK := lineMap[source.LineID]
		templateID, templateOK := templateMap[source.TemplateID]
		versionID, versionOK := versionMap[source.ActiveVersionID]
		if !lineOK || !templateOK || !versionOK {
			return nil, nil, errors.New("产线表单分配引用的数据不完整")
		}
		assignment := source
		assignment.ID = 0
		assignment.LineID = lineID
		assignment.TemplateID = templateID
		assignment.ActiveVersionID = versionID
		if err := tx.Create(&assignment).Error; err != nil {
			return nil, nil, err
		}
	}
	return templateMap, versionMap, nil
}

func fillMissingFormReferences(tx *gorm.DB, departmentID uint64, assignments []model.LineFormAssignment, submissions []model.Submission, templateMap, versionMap, userMap map[uint64]uint64, result *RestoreResult) error {
	requiredTemplates := map[uint64]bool{}
	requiredVersions := map[uint64]bool{}
	versionTemplate := map[uint64]uint64{}
	for _, assignment := range assignments {
		requiredTemplates[assignment.TemplateID] = true
		requiredVersions[assignment.ActiveVersionID] = true
		versionTemplate[assignment.ActiveVersionID] = assignment.TemplateID
	}
	for _, submission := range submissions {
		requiredTemplates[submission.TemplateID] = true
		requiredVersions[submission.VersionID] = true
		versionTemplate[submission.VersionID] = submission.TemplateID
	}
	if len(requiredTemplates) == 0 && len(requiredVersions) == 0 {
		return nil
	}
	createdBy := uint64(0)
	for _, userID := range userMap {
		createdBy = userID
		break
	}
	if createdBy == 0 {
		return errors.New("缺少可用于恢复历史表单的用户")
	}
	for oldID := range requiredTemplates {
		if _, ok := templateMap[oldID]; ok {
			continue
		}
		template := model.FormTemplate{
			DepartmentID: departmentID, Code: fmt.Sprintf("__restored_template_%d", oldID),
			Name: fmt.Sprintf("历史模板%d", oldID), Kind: model.FormKindReport, CreatedBy: createdBy,
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		}
		if err := tx.Create(&template).Error; err != nil {
			return err
		}
		templateMap[oldID] = template.ID
		result.FormTemplates++
	}
	nextVersion := map[uint64]int{}
	for oldVersionID := range requiredVersions {
		if _, ok := versionMap[oldVersionID]; ok {
			continue
		}
		oldTemplateID, ok := versionTemplate[oldVersionID]
		if !ok {
			return fmt.Errorf("历史版本 %d 缺少模板信息", oldVersionID)
		}
		templateID, ok := templateMap[oldTemplateID]
		if !ok {
			return fmt.Errorf("历史版本 %d 的模板无法恢复", oldVersionID)
		}
		number := nextVersion[templateID]
		if number == 0 {
			var maxVersion int
			_ = tx.Model(&model.FormVersion{}).Where("template_id = ?", templateID).Select("COALESCE(MAX(version_no),0)").Scan(&maxVersion).Error
			number = maxVersion + 1
		}
		nextVersion[templateID] = number + 1
		version := model.FormVersion{
			TemplateID: templateID, VersionNo: number,
			Schema: datatypes.JSON([]byte(`{"fields":[]}`)), Status: model.VersionRetired, CreatedBy: createdBy,
		}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		versionMap[oldVersionID] = version.ID
		result.FormVersions++
	}
	return nil
}

func restoreSubmissions(tx *gorm.DB, departmentID uint64, submissions []model.Submission, rows []model.SubmissionRow, values []model.FieldValue, templateMap, versionMap, lineMap, shiftItemMap, userMap map[uint64]uint64, result *RestoreResult) (map[uint64]uint64, map[uint64]uint64, error) {
	submissionMap := make(map[uint64]uint64, len(submissions))
	for _, source := range submissions {
		templateID, ok := templateMap[source.TemplateID]
		if !ok {
			return nil, nil, errors.New("填报引用的模板不完整")
		}
		versionID, ok := versionMap[source.VersionID]
		if !ok {
			return nil, nil, errors.New("填报引用的版本不完整")
		}
		lineID, ok := lineMap[source.LineID]
		if !ok {
			return nil, nil, errors.New("填报引用的产线不完整")
		}
		shiftItemID, ok := shiftItemMap[source.ShiftPlanItemID]
		if !ok {
			return nil, nil, errors.New("填报引用的班次不完整")
		}
		createdBy, ok := userMap[source.CreatedBy]
		if !ok {
			return nil, nil, errors.New("填报引用的用户不完整")
		}
		submission := source
		submission.ID = 0
		submission.DepartmentID = departmentID
		submission.TemplateID = templateID
		submission.VersionID = versionID
		submission.LineID = lineID
		submission.ShiftPlanItemID = shiftItemID
		submission.CreatedBy = createdBy
		if err := tx.Create(&submission).Error; err != nil {
			return nil, nil, err
		}
		submissionMap[source.ID] = submission.ID
		result.Submissions++
	}
	rowMap := make(map[uint64]uint64, len(rows))
	for _, source := range rows {
		submissionID, ok := submissionMap[source.SubmissionID]
		if !ok {
			return nil, nil, errors.New("填报明细引用的填报不完整")
		}
		operatorID, ok := userMap[source.OperatorID]
		if !ok {
			return nil, nil, errors.New("填报明细引用的操作人员不完整")
		}
		row := source
		row.ID = 0
		row.SubmissionID = submissionID
		row.OperatorID = operatorID
		if row.VoidedBy != nil {
			mapped, ok := userMap[*row.VoidedBy]
			if !ok {
				return nil, nil, errors.New("填报明细废除用户不完整")
			}
			row.VoidedBy = &mapped
		}
		if row.RestoredBy != nil {
			mapped, ok := userMap[*row.RestoredBy]
			if !ok {
				return nil, nil, errors.New("填报明细恢复用户不完整")
			}
			row.RestoredBy = &mapped
		}
		if err := tx.Create(&row).Error; err != nil {
			return nil, nil, err
		}
		rowMap[source.ID] = row.ID
		result.SubmissionRows++
	}
	for _, source := range values {
		submissionID, ok := submissionMap[source.SubmissionID]
		if !ok {
			return nil, nil, errors.New("字段值引用的填报不完整")
		}
		createdBy, ok := userMap[source.CreatedBy]
		if !ok {
			return nil, nil, errors.New("字段值引用的用户不完整")
		}
		value := source
		value.ID = 0
		value.SubmissionID = submissionID
		value.CreatedBy = createdBy
		if value.RowID != nil {
			mapped, ok := rowMap[*value.RowID]
			if !ok {
				return nil, nil, errors.New("字段值引用的明细不完整")
			}
			value.RowID = &mapped
		}
		if err := tx.Create(&value).Error; err != nil {
			return nil, nil, err
		}
		result.FieldValues++
	}
	return submissionMap, rowMap, nil
}

func restoreAuditAndInstrument(tx *gorm.DB, departmentID uint64, logs []model.AuditLog, messages []model.InstrumentMessage, userMap map[uint64]uint64, result *RestoreResult) error {
	for _, source := range logs {
		log := source
		log.ID = 0
		log.DepartmentID = &departmentID
		if mapped, ok := userMap[source.ActorID]; ok {
			log.ActorID = mapped
		} else {
			log.ActorID = 0
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		result.AuditLogs++
	}
	for _, source := range messages {
		message := source
		message.ID = 0
		message.DepartmentID = &departmentID
		if err := tx.Create(&message).Error; err != nil {
			return err
		}
		result.InstrumentMsgs++
	}
	return nil
}

func restoreDepartmentSettings(tx *gorm.DB, departmentID uint64, settings []model.DepartmentSetting, result *RestoreResult) error {
	now := time.Now()
	for _, source := range settings {
		setting := source
		setting.DepartmentID = departmentID
		if setting.UpdatedAt.IsZero() {
			setting.UpdatedAt = now
		}
		if err := tx.Create(&setting).Error; err != nil {
			return err
		}
	}
	return nil
}
