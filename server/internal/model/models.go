// 文件说明：系统领域模型定义，同时声明 GORM 表映射和 API JSON 字段。

package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// 业务状态常量集中定义，避免处理器和服务层散落字符串字面量。
const (
	FormKindFirstInspection = "first_inspection"
	FormKindFinalInspection = "final_inspection"
	FormKindReport          = "report"
	FormKindRegistration    = "registration"

	VersionDraft     = "draft"
	VersionPublished = "published"
	VersionRetired   = "retired"

	SubmissionStatusActive = "active"
	SubmissionStatusVoided = "voided"

	BackupKindFull       = "full"
	BackupKindDepartment = "department"
)

// Department 表示数据隔离边界和部门级设置归属。
type Department struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	Code      string         `gorm:"size:64;index;not null" json:"code"`
	Name      string         `gorm:"size:128;not null" json:"name"`
	Enabled   bool           `gorm:"default:true;not null" json:"enabled"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// User 保存登录账号、密码散列、部门、角色和启停状态。
type User struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	Account      string         `gorm:"size:64;index;not null" json:"account"`
	Name         string         `gorm:"size:128;not null" json:"name"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	DepartmentID *uint64        `gorm:"index" json:"departmentId,omitempty"`
	Department   *Department    `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	RoleID       uint64         `gorm:"index;not null" json:"roleId"`
	Role         Role           `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Enabled      bool           `gorm:"default:true;not null" json:"enabled"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Role 描述角色权限集合和允许登录的客户端类型。
type Role struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	Code      string         `gorm:"size:64;index;not null" json:"code"`
	Name      string         `gorm:"size:128;not null" json:"name"`
	IsSystem  bool           `gorm:"default:false;not null" json:"isSystem"`
	AllowPC   bool           `gorm:"default:false;not null" json:"allowPc"`
	AllowPDA  bool           `gorm:"default:true;not null" json:"allowPda"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Permission struct {
	ID    uint64 `gorm:"primaryKey" json:"id"`
	Code  string `gorm:"size:96;uniqueIndex;not null" json:"code"`
	Name  string `gorm:"size:128;not null" json:"name"`
	Group string `gorm:"size:64;index" json:"group"`
}

type RolePermission struct {
	RoleID       uint64 `gorm:"primaryKey" json:"roleId"`
	PermissionID uint64 `gorm:"primaryKey" json:"permissionId"`
}

type RoleLineScope struct {
	RoleID uint64 `gorm:"primaryKey" json:"roleId"`
	LineID uint64 `gorm:"primaryKey" json:"lineId"`
}

// Line 表示最多两级的产线树。
type Line struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	DepartmentID uint64         `gorm:"index;not null" json:"departmentId"`
	Department   Department     `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	ParentID     *uint64        `gorm:"index" json:"parentId"`
	Code         string         `gorm:"size:64;index;not null" json:"code"`
	Name         string         `gorm:"size:128;not null" json:"name"`
	Level        int            `gorm:"not null" json:"level"`
	Enabled      bool           `gorm:"default:true;not null" json:"enabled"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Product 表示产品基础数据，并通过 product_lines 关联可用产线。
type Product struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	DepartmentID uint64         `gorm:"index;not null" json:"departmentId"`
	Department   Department     `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Code         string         `gorm:"size:64;index;not null" json:"code"`
	Name         string         `gorm:"size:128;not null" json:"name"`
	Enabled      bool           `gorm:"default:true;not null" json:"enabled"`
	Lines        []Line         `gorm:"many2many:product_lines" json:"lines,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// ShiftPlan 表示一个按生效时间选择的班次计划。
type ShiftPlan struct {
	ID           uint64          `gorm:"primaryKey" json:"id"`
	DepartmentID uint64          `gorm:"index;not null" json:"departmentId"`
	Department   Department      `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Name         string          `gorm:"size:128;not null" json:"name"`
	EffectiveAt  time.Time       `gorm:"index;not null" json:"effectiveAt"`
	CreatedBy    uint64          `json:"createdBy"`
	Items        []ShiftPlanItem `gorm:"foreignKey:PlanID" json:"items,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt  `gorm:"index" json:"-"`
}

type ShiftPlanItem struct {
	ID            uint64 `gorm:"primaryKey" json:"id"`
	PlanID        uint64 `gorm:"index;not null" json:"planId"`
	Name          string `gorm:"size:64;not null" json:"name"`
	StartTime     string `gorm:"size:5;not null" json:"startTime"`
	EndTime       string `gorm:"size:5;not null" json:"endTime"`
	CrossMidnight bool   `gorm:"default:false;not null" json:"crossMidnight"`
	SortOrder     int    `gorm:"default:0" json:"sortOrder"`
}

// FormTemplate 表示动态表单模板，版本通过 FormVersion 保存。
type FormTemplate struct {
	ID               uint64         `gorm:"primaryKey" json:"id"`
	DepartmentID     uint64         `gorm:"index;not null" json:"departmentId"`
	Department       Department     `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Code             string         `gorm:"size:96;index;not null" json:"code"`
	Name             string         `gorm:"size:128;not null" json:"name"`
	Kind             string         `gorm:"size:32;index;not null" json:"kind"`
	PairedTemplateID *uint64        `gorm:"index" json:"pairedTemplateId"`
	CreatedBy        uint64         `json:"createdBy"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// FormVersion 保存 JSONB Schema、版本号和发布状态。
type FormVersion struct {
	ID          uint64         `gorm:"primaryKey" json:"id"`
	TemplateID  uint64         `gorm:"index;not null" json:"templateId"`
	VersionNo   int            `gorm:"not null" json:"versionNo"`
	Schema      datatypes.JSON `gorm:"type:jsonb;not null" json:"schema"`
	Status      string         `gorm:"size:32;index;not null" json:"status"`
	PublishedAt *time.Time     `json:"publishedAt"`
	CreatedBy   uint64         `json:"createdBy"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// LineFormAssignment 保存产线当前启用的模板版本。
type LineFormAssignment struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	LineID          uint64    `gorm:"uniqueIndex:idx_line_template;not null" json:"lineId"`
	TemplateID      uint64    `gorm:"uniqueIndex:idx_line_template;not null" json:"templateId"`
	ActiveVersionID uint64    `gorm:"index;not null" json:"activeVersionId"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// Submission 表示一次完整填报，并保存部门、模板版本和班次快照。
type Submission struct {
	ID                 uint64         `gorm:"primaryKey" json:"id"`
	DepartmentID       uint64         `gorm:"index;not null" json:"departmentId"`
	Department         Department     `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	TemplateID         uint64         `gorm:"index;not null" json:"templateId"`
	VersionID          uint64         `gorm:"index;not null" json:"versionId"`
	LineID             uint64         `gorm:"index;not null" json:"lineId"`
	BusinessDate       time.Time      `gorm:"type:date;index;not null" json:"businessDate"`
	ShiftPlanItemID    uint64         `gorm:"index;not null" json:"shiftPlanItemId"`
	ShiftNameSnapshot  string         `gorm:"size:64;not null" json:"shiftNameSnapshot"`
	ShiftStartSnapshot string         `gorm:"size:5;not null" json:"shiftStartSnapshot"`
	ShiftEndSnapshot   string         `gorm:"size:5;not null" json:"shiftEndSnapshot"`
	RepeatReason       string         `gorm:"type:text" json:"repeatReason"`
	Source             string         `gorm:"size:32;default:online" json:"source"`
	ClientTime         *time.Time     `json:"clientTime"`
	CreatedBy          uint64         `gorm:"index;not null" json:"createdBy"`
	SubmittedAt        time.Time      `gorm:"index;not null" json:"submittedAt"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// SubmissionRow 表示填报中的一行，可独立废除和恢复。
type SubmissionRow struct {
	ID            uint64         `gorm:"primaryKey" json:"id"`
	SubmissionID  uint64         `gorm:"index;not null" json:"submissionId"`
	RowNo         int            `gorm:"not null" json:"rowNo"`
	OperatorID    uint64         `gorm:"index;not null" json:"operatorId"`
	OperatedAt    time.Time      `gorm:"index;not null" json:"operatedAt"`
	Status        string         `gorm:"size:32;index;default:active;not null" json:"status"`
	VoidReason    string         `gorm:"type:text" json:"voidReason"`
	RestoreReason string         `gorm:"type:text" json:"restoreReason"`
	VoidedBy      *uint64        `json:"voidedBy"`
	VoidedAt      *time.Time     `json:"voidedAt"`
	RestoredBy    *uint64        `json:"restoredBy"`
	RestoredAt    *time.Time     `json:"restoredAt"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// FieldValue 保存字段值，同时保留文本值和数值索引。
type FieldValue struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	SubmissionID uint64    `gorm:"index;not null" json:"submissionId"`
	RowID        *uint64   `gorm:"index" json:"rowId"`
	FieldKey     string    `gorm:"size:96;index;not null" json:"fieldKey"`
	ValueType    string    `gorm:"size:32;not null" json:"valueType"`
	ValueText    string    `gorm:"type:text" json:"valueText"`
	ValueNumber  *float64  `json:"valueNumber"`
	RepeatIndex  int       `gorm:"default:0" json:"repeatIndex"`
	Source       string    `gorm:"size:32;default:manual" json:"source"`
	CreatedBy    uint64    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AuditLog 保存关键动作前后快照和请求追踪信息。
type AuditLog struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	DepartmentID *uint64        `gorm:"index" json:"departmentId,omitempty"`
	Department   *Department    `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	EntityType   string         `gorm:"size:64;index;not null" json:"entityType"`
	EntityID     uint64         `gorm:"index;not null" json:"entityId"`
	Action       string         `gorm:"size:64;index;not null" json:"action"`
	BeforeJSON   datatypes.JSON `gorm:"type:jsonb" json:"before"`
	AfterJSON    datatypes.JSON `gorm:"type:jsonb" json:"after"`
	ActorID      uint64         `gorm:"index" json:"actorId"`
	IP           string         `gorm:"size:64" json:"ip"`
	RequestID    string         `gorm:"size:64;index" json:"requestId"`
	CreatedAt    time.Time      `gorm:"index" json:"createdAt"`
}

// InstrumentMessage 保存仪表原始报文，供后续解析扩展。
type InstrumentMessage struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	DepartmentID *uint64        `gorm:"index" json:"departmentId,omitempty"`
	Department   *Department    `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Topic        string         `gorm:"size:255;index" json:"topic"`
	DeviceID     string         `gorm:"size:128;index" json:"deviceId"`
	RawJSON      datatypes.JSON `gorm:"type:jsonb;not null" json:"rawJson"`
	ReceivedAt   time.Time      `gorm:"index;not null" json:"receivedAt"`
	Processed    bool           `gorm:"default:false;index" json:"processed"`
}

type AppSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DepartmentSetting struct {
	DepartmentID uint64    `gorm:"primaryKey" json:"departmentId"`
	Key          string    `gorm:"primaryKey;size:64" json:"key"`
	Value        string    `gorm:"type:text;not null" json:"value"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// BackupRecord 记录完整备份或部门归档文件。
type BackupRecord struct {
	ID           uint64      `gorm:"primaryKey" json:"id"`
	Kind         string      `gorm:"size:32;index;not null;default:full" json:"kind"`
	DepartmentID *uint64     `gorm:"index" json:"departmentId,omitempty"`
	Department   *Department `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	FileName     string      `gorm:"size:255;not null" json:"fileName"`
	FilePath     string      `gorm:"size:512;not null" json:"filePath"`
	FileSize     int64       `json:"fileSize"`
	CreatedBy    uint64      `gorm:"index;not null" json:"createdBy"`
	CreatedAt    time.Time   `json:"createdAt"`
}
