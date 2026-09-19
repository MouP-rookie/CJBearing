package backup

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bearing-inspection/internal/model"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	pgDump string
	dir    string
}

type archiveUser struct {
	ID           uint64    `json:"id"`
	Account      string    `json:"account"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"passwordHash"`
	DepartmentID *uint64   `json:"departmentId,omitempty"`
	RoleID       uint64    `json:"roleId"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	Deleted      bool      `json:"deleted,omitempty"`
}

type departmentArchive struct {
	Department          model.Department           `json:"department"`
	Settings            []model.DepartmentSetting  `json:"settings"`
	Users               []archiveUser              `json:"users"`
	Lines               []model.Line               `json:"lines"`
	Products            []model.Product            `json:"products"`
	ShiftPlans          []model.ShiftPlan          `json:"shiftPlans"`
	FormTemplates       []model.FormTemplate       `json:"formTemplates"`
	FormVersions        []model.FormVersion        `json:"formVersions"`
	LineFormAssignments []model.LineFormAssignment `json:"lineFormAssignments"`
	Submissions         []model.Submission         `json:"submissions"`
	SubmissionRows      []model.SubmissionRow      `json:"submissionRows"`
	FieldValues         []model.FieldValue         `json:"fieldValues"`
	AuditLogs           []model.AuditLog           `json:"auditLogs"`
	InstrumentMessages  []model.InstrumentMessage  `json:"instrumentMessages"`
	DeletedUserIDs      []uint64                   `json:"deletedUserIds,omitempty"`
	DeletedLineIDs      []uint64                   `json:"deletedLineIds,omitempty"`
	DeletedShiftPlanIDs []uint64                   `json:"deletedShiftPlanIds,omitempty"`
	DeletedTemplateIDs  []uint64                   `json:"deletedTemplateIds,omitempty"`
}

type archiveManifest struct {
	Version      int       `json:"version"`
	GeneratedAt  time.Time `json:"generatedAt"`
	DepartmentID uint64    `json:"departmentId"`
	Department   string    `json:"department"`
}

func New(db *gorm.DB, pgDump, dir string) *Service {
	return &Service{db: db, pgDump: pgDump, dir: dir}
}

func (s *Service) Create(databaseURL string, actorID uint64) (*model.BackupRecord, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("dg_inspection_%s.dump", time.Now().Format("20060102_150405"))
	path := filepath.Join(s.dir, name)
	backupURL := databaseURL
	if u, err := url.Parse(databaseURL); err == nil {
		q := u.Query()
		q.Del("timezone")
		q.Del("TimeZone")
		u.RawQuery = q.Encode()
		backupURL = u.String()
	}
	cmd := exec.Command(s.pgDump, "--format=custom", "--file", path, "--dbname", backupURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pg_dump: %w: %s", err, string(out))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	rec := &model.BackupRecord{Kind: model.BackupKindFull, FileName: name, FilePath: path, FileSize: info.Size(), CreatedBy: actorID}
	if err := s.db.Create(rec).Error; err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Service) CreateDepartmentArchive(departmentID, actorID uint64) (*model.BackupRecord, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	var department model.Department
	if err := s.db.First(&department, departmentID).Error; err != nil {
		return nil, err
	}
	data, err := s.loadDepartmentArchive(department)
	if err != nil {
		return nil, err
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}
	manifest, err := json.MarshalIndent(archiveManifest{Version: 1, GeneratedAt: time.Now(), DepartmentID: department.ID, Department: department.Name}, "", "  ")
	if err != nil {
		return nil, err
	}
	safeCode := regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(department.Code, "_")
	if strings.TrimSpace(safeCode) == "" {
		safeCode = fmt.Sprintf("department_%d", department.ID)
	}
	name := fmt.Sprintf("dept_%s_%s.zip", safeCode, time.Now().Format("20060102_150405"))
	path := filepath.Join(s.dir, name)
	if err := writeArchive(path, map[string][]byte{"manifest.json": manifest, "data.json": payload}); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	rec := &model.BackupRecord{Kind: model.BackupKindDepartment, DepartmentID: &department.ID, FileName: name, FilePath: path, FileSize: info.Size(), CreatedBy: actorID}
	if err := s.db.Create(rec).Error; err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Service) loadDepartmentArchive(department model.Department) (*departmentArchive, error) {
	data := &departmentArchive{Department: department}
	if err := s.db.Where("department_id = ?", department.ID).Order("key").Find(&data.Settings).Error; err != nil {
		return nil, err
	}
	var users []model.User
	if err := s.db.Where("department_id = ?", department.ID).Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	data.Users = make([]archiveUser, 0, len(users))
	for _, user := range users {
		data.Users = append(data.Users, archiveUser{
			ID: user.ID, Account: user.Account, Name: user.Name, PasswordHash: user.PasswordHash,
			DepartmentID: user.DepartmentID, RoleID: user.RoleID, Enabled: user.Enabled, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
		})
	}
	if err := s.db.Where("department_id = ?", department.ID).Order("id").Find(&data.Lines).Error; err != nil {
		return nil, err
	}
	if err := s.db.Preload("Lines", "department_id = ?", department.ID).Where("department_id = ?", department.ID).Order("id").Find(&data.Products).Error; err != nil {
		return nil, err
	}
	if err := s.db.Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order, id") }).Where("department_id = ?", department.ID).Order("id").Find(&data.ShiftPlans).Error; err != nil {
		return nil, err
	}
	if err := s.db.Where("department_id = ?", department.ID).Order("id").Find(&data.FormTemplates).Error; err != nil {
		return nil, err
	}
	templateIDs := make([]uint64, 0, len(data.FormTemplates))
	for _, template := range data.FormTemplates {
		templateIDs = append(templateIDs, template.ID)
	}
	lineIDs := make([]uint64, 0, len(data.Lines))
	for _, line := range data.Lines {
		lineIDs = append(lineIDs, line.ID)
	}
	if len(templateIDs) > 0 {
		if err := s.db.Where("template_id IN ?", templateIDs).Order("template_id, version_no").Find(&data.FormVersions).Error; err != nil {
			return nil, err
		}
		if len(lineIDs) > 0 {
			if err := s.db.Where("template_id IN ? AND line_id IN ?", templateIDs, lineIDs).Order("id").Find(&data.LineFormAssignments).Error; err != nil {
				return nil, err
			}
		}
	}
	if err := s.db.Where("department_id = ?", department.ID).Order("id").Find(&data.Submissions).Error; err != nil {
		return nil, err
	}
	submissionIDs := make([]uint64, 0, len(data.Submissions))
	for _, submission := range data.Submissions {
		submissionIDs = append(submissionIDs, submission.ID)
	}
	if len(submissionIDs) > 0 {
		if err := s.db.Where("submission_id IN ?", submissionIDs).Order("submission_id, row_no").Find(&data.SubmissionRows).Error; err != nil {
			return nil, err
		}
		if err := s.db.Where("submission_id IN ?", submissionIDs).Order("submission_id, row_id, repeat_index, id").Find(&data.FieldValues).Error; err != nil {
			return nil, err
		}
	}
	if err := s.db.Where("department_id = ?", department.ID).Order("id").Find(&data.AuditLogs).Error; err != nil {
		return nil, err
	}
	if err := s.db.Where("department_id = ?", department.ID).Order("id").Find(&data.InstrumentMessages).Error; err != nil {
		return nil, err
	}
	if err := s.addMissingArchiveReferences(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Service) addMissingArchiveReferences(data *departmentArchive) error {
	userIDs := map[uint64]bool{}
	for _, user := range data.Users {
		userIDs[user.ID] = true
	}
	requiredUsers := map[uint64]bool{}
	for _, submission := range data.Submissions {
		requiredUsers[submission.CreatedBy] = true
	}
	for _, row := range data.SubmissionRows {
		requiredUsers[row.OperatorID] = true
		if row.VoidedBy != nil {
			requiredUsers[*row.VoidedBy] = true
		}
		if row.RestoredBy != nil {
			requiredUsers[*row.RestoredBy] = true
		}
	}
	for _, value := range data.FieldValues {
		requiredUsers[value.CreatedBy] = true
	}
	var missingUserIDs []uint64
	for id := range requiredUsers {
		if !userIDs[id] {
			missingUserIDs = append(missingUserIDs, id)
		}
	}
	if len(missingUserIDs) > 0 {
		var users []model.User
		if err := s.db.Unscoped().Where("id IN ?", missingUserIDs).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			data.Users = append(data.Users, archiveUser{
				ID: user.ID, Account: user.Account, Name: user.Name, PasswordHash: user.PasswordHash,
				DepartmentID: user.DepartmentID, RoleID: user.RoleID, Enabled: user.Enabled,
				CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, Deleted: user.DeletedAt.Valid,
			})
			if user.DeletedAt.Valid {
				data.DeletedUserIDs = append(data.DeletedUserIDs, user.ID)
			}
		}
	}

	lineIDs := map[uint64]bool{}
	for _, line := range data.Lines {
		lineIDs[line.ID] = true
	}
	requiredLines := map[uint64]bool{}
	for _, submission := range data.Submissions {
		requiredLines[submission.LineID] = true
	}
	for _, assignment := range data.LineFormAssignments {
		requiredLines[assignment.LineID] = true
	}
	for len(requiredLines) > 0 {
		var missing []uint64
		for id := range requiredLines {
			if !lineIDs[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			break
		}
		var lines []model.Line
		if err := s.db.Unscoped().Where("id IN ?", missing).Find(&lines).Error; err != nil {
			return err
		}
		if len(lines) == 0 {
			break
		}
		for _, line := range lines {
			if lineIDs[line.ID] {
				continue
			}
			lineIDs[line.ID] = true
			data.Lines = append(data.Lines, line)
			if line.DeletedAt.Valid {
				data.DeletedLineIDs = append(data.DeletedLineIDs, line.ID)
			}
			if line.ParentID != nil {
				requiredLines[*line.ParentID] = true
			}
		}
		for id := range requiredLines {
			if !lineIDs[id] {
				continue
			}
			delete(requiredLines, id)
		}
	}

	templateIDs := map[uint64]bool{}
	for _, template := range data.FormTemplates {
		templateIDs[template.ID] = true
	}
	requiredTemplates := map[uint64]bool{}
	for _, submission := range data.Submissions {
		requiredTemplates[submission.TemplateID] = true
	}
	for _, assignment := range data.LineFormAssignments {
		requiredTemplates[assignment.TemplateID] = true
	}
	for len(requiredTemplates) > 0 {
		var missing []uint64
		for id := range requiredTemplates {
			if !templateIDs[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			break
		}
		var templates []model.FormTemplate
		if err := s.db.Unscoped().Where("id IN ?", missing).Find(&templates).Error; err != nil {
			return err
		}
		if len(templates) == 0 {
			break
		}
		for _, template := range templates {
			if templateIDs[template.ID] {
				continue
			}
			templateIDs[template.ID] = true
			data.FormTemplates = append(data.FormTemplates, template)
			if template.DeletedAt.Valid {
				data.DeletedTemplateIDs = append(data.DeletedTemplateIDs, template.ID)
			}
			if template.PairedTemplateID != nil {
				requiredTemplates[*template.PairedTemplateID] = true
			}
		}
		for id := range requiredTemplates {
			if templateIDs[id] {
				delete(requiredTemplates, id)
			}
		}
	}

	versionIDs := map[uint64]bool{}
	for _, version := range data.FormVersions {
		versionIDs[version.ID] = true
	}
	requiredVersions := map[uint64]bool{}
	for _, submission := range data.Submissions {
		requiredVersions[submission.VersionID] = true
	}
	for _, assignment := range data.LineFormAssignments {
		requiredVersions[assignment.ActiveVersionID] = true
	}
	var missingVersionIDs []uint64
	for id := range requiredVersions {
		if !versionIDs[id] {
			missingVersionIDs = append(missingVersionIDs, id)
		}
	}
	if len(missingVersionIDs) > 0 {
		var versions []model.FormVersion
		if err := s.db.Where("id IN ?", missingVersionIDs).Find(&versions).Error; err != nil {
			return err
		}
		data.FormVersions = append(data.FormVersions, versions...)
	}

	planIDs := map[uint64]bool{}
	itemIDs := map[uint64]bool{}
	for _, plan := range data.ShiftPlans {
		planIDs[plan.ID] = true
		for _, item := range plan.Items {
			itemIDs[item.ID] = true
		}
	}
	requiredItems := map[uint64]bool{}
	for _, submission := range data.Submissions {
		requiredItems[submission.ShiftPlanItemID] = true
	}
	var missingItemIDs []uint64
	for id := range requiredItems {
		if !itemIDs[id] {
			missingItemIDs = append(missingItemIDs, id)
		}
	}
	if len(missingItemIDs) > 0 {
		var items []model.ShiftPlanItem
		if err := s.db.Where("id IN ?", missingItemIDs).Find(&items).Error; err != nil {
			return err
		}
		var missingPlanIDs []uint64
		for _, item := range items {
			if !planIDs[item.PlanID] {
				missingPlanIDs = append(missingPlanIDs, item.PlanID)
			}
		}
		if len(missingPlanIDs) > 0 {
			var plans []model.ShiftPlan
			if err := s.db.Unscoped().Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order, id") }).Where("id IN ?", missingPlanIDs).Find(&plans).Error; err != nil {
				return err
			}
			for _, plan := range plans {
				if planIDs[plan.ID] {
					continue
				}
				planIDs[plan.ID] = true
				data.ShiftPlans = append(data.ShiftPlans, plan)
				if plan.DeletedAt.Valid {
					data.DeletedShiftPlanIDs = append(data.DeletedShiftPlanIDs, plan.ID)
				}
				for _, item := range plan.Items {
					itemIDs[item.ID] = true
				}
			}
		}
	}
	return nil
}

func writeArchive(path string, files map[string][]byte) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	writer := zip.NewWriter(out)
	for name, payload := range files {
		entry, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write(payload); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return out.Close()
}

func (s *Service) List(departmentID uint64, allDepartments bool) ([]model.BackupRecord, error) {
	var out []model.BackupRecord
	q := s.db.Preload("Department").Order("created_at desc")
	if !allDepartments {
		q = q.Where("kind = ? AND department_id = ?", model.BackupKindDepartment, departmentID)
	}
	return out, q.Find(&out).Error
}

func (s *Service) ListPage(departmentID uint64, allDepartments bool, page, size int) ([]model.BackupRecord, int64, error) {
	buildQuery := func() *gorm.DB {
		q := s.db.Model(&model.BackupRecord{})
		if !allDepartments {
			q = q.Where("kind = ? AND department_id = ?", model.BackupKindDepartment, departmentID)
		}
		return q
	}
	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []model.BackupRecord
	err := buildQuery().Preload("Department").Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&out).Error
	return out, total, err
}

func (s *Service) Get(id uint64) (*model.BackupRecord, error) {
	var out model.BackupRecord
	if err := s.db.Preload("Department").First(&out, id).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) GetForScope(id, departmentID uint64, allDepartments bool) (*model.BackupRecord, error) {
	var out model.BackupRecord
	q := s.db.Preload("Department")
	if !allDepartments {
		q = q.Where("kind = ? AND department_id = ?", model.BackupKindDepartment, departmentID)
	}
	if err := q.First(&out, id).Error; err != nil {
		return nil, err
	}
	return &out, nil
}
