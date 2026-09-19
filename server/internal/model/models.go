package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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
)

type User struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	Account      string         `gorm:"size:64;index;not null" json:"account"`
	Name         string         `gorm:"size:128;not null" json:"name"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	RoleID       uint64         `gorm:"index;not null" json:"roleId"`
	Role         Role           `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Enabled      bool           `gorm:"default:true;not null" json:"enabled"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

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

type Line struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	ParentID  *uint64        `gorm:"index" json:"parentId"`
	Code      string         `gorm:"size:64;index;not null" json:"code"`
	Name      string         `gorm:"size:128;not null" json:"name"`
	Level     int            `gorm:"not null" json:"level"`
	Enabled   bool           `gorm:"default:true;not null" json:"enabled"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Product struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	Code      string         `gorm:"size:64;index;not null" json:"code"`
	Name      string         `gorm:"size:128;not null" json:"name"`
	Enabled   bool           `gorm:"default:true;not null" json:"enabled"`
	Lines     []Line         `gorm:"many2many:product_lines" json:"lines,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
type ShiftPlan struct {
	ID          uint64          `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"size:128;not null" json:"name"`
	EffectiveAt time.Time       `gorm:"index;not null" json:"effectiveAt"`
	CreatedBy   uint64          `json:"createdBy"`
	Items       []ShiftPlanItem `gorm:"foreignKey:PlanID" json:"items,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
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

type FormTemplate struct {
	ID               uint64         `gorm:"primaryKey" json:"id"`
	Code             string         `gorm:"size:96;index;not null" json:"code"`
	Name             string         `gorm:"size:128;not null" json:"name"`
	Kind             string         `gorm:"size:32;index;not null" json:"kind"`
	PairedTemplateID *uint64        `gorm:"index" json:"pairedTemplateId"`
	CreatedBy        uint64         `json:"createdBy"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

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

type LineFormAssignment struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	LineID          uint64    `gorm:"uniqueIndex:idx_line_template;not null" json:"lineId"`
	TemplateID      uint64    `gorm:"uniqueIndex:idx_line_template;not null" json:"templateId"`
	ActiveVersionID uint64    `gorm:"index;not null" json:"activeVersionId"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Submission struct {
	ID                 uint64         `gorm:"primaryKey" json:"id"`
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

type AuditLog struct {
	ID         uint64         `gorm:"primaryKey" json:"id"`
	EntityType string         `gorm:"size:64;index;not null" json:"entityType"`
	EntityID   uint64         `gorm:"index;not null" json:"entityId"`
	Action     string         `gorm:"size:64;index;not null" json:"action"`
	BeforeJSON datatypes.JSON `gorm:"type:jsonb" json:"before"`
	AfterJSON  datatypes.JSON `gorm:"type:jsonb" json:"after"`
	ActorID    uint64         `gorm:"index" json:"actorId"`
	IP         string         `gorm:"size:64" json:"ip"`
	RequestID  string         `gorm:"size:64;index" json:"requestId"`
	CreatedAt  time.Time      `gorm:"index" json:"createdAt"`
}

type InstrumentMessage struct {
	ID         uint64         `gorm:"primaryKey" json:"id"`
	Topic      string         `gorm:"size:255;index" json:"topic"`
	DeviceID   string         `gorm:"size:128;index" json:"deviceId"`
	RawJSON    datatypes.JSON `gorm:"type:jsonb;not null" json:"rawJson"`
	ReceivedAt time.Time      `gorm:"index;not null" json:"receivedAt"`
	Processed  bool           `gorm:"default:false;index" json:"processed"`
}

type AppSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type BackupRecord struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	FileName  string    `gorm:"size:255;not null" json:"fileName"`
	FilePath  string    `gorm:"size:512;not null" json:"filePath"`
	FileSize  int64     `json:"fileSize"`
	CreatedBy uint64    `gorm:"index;not null" json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}
