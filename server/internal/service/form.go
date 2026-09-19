// 文件说明：动态表单服务，负责 Schema 校验、默认变量和产品字段处理。

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"bearing-inspection/internal/model"
)

type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type FieldDefinition struct {
	Key           string        `json:"key"`
	Label         string        `json:"label"`
	Type          string        `json:"type"`
	Required      bool          `json:"required"`
	Options       []FieldOption `json:"options,omitempty"`
	DefaultValue  string        `json:"defaultValue,omitempty"`
	Unit          string        `json:"unit,omitempty"`
	Min           *float64      `json:"min,omitempty"`
	Max           *float64      `json:"max,omitempty"`
	AllowMultiple bool          `json:"allowMultiple,omitempty"`
	Repeatable    bool          `json:"repeatable,omitempty"`
	SortOrder     int           `json:"sortOrder"`
}

type FormSchema struct {
	Fields []FieldDefinition `json:"fields"`
}

type DefaultContext struct {
	Date        string
	Time        string
	Shift       string
	UserName    string
	UserAccount string
	LineName    string
	LineProduct string
	FormName    string
	Stage       string
	Now         time.Time
}

var tokenPattern = regexp.MustCompile(`\{\{([a-z_]+)\}\}`)

// ParseSchema 先反序列化，再执行统一结构校验。
func ParseSchema(raw []byte) (FormSchema, error) {
	var schema FormSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return schema, err
	}
	return schema, ValidateSchema(schema)
}

// ValidateSchema 检查字段 key、类型、选项、数值上下限和产品字段约束。
func ValidateSchema(schema FormSchema) error {
	if len(schema.Fields) == 0 {
		return errors.New("表单至少需要一个字段")
	}
	seen := map[string]bool{}
	allowed := map[string]bool{"text": true, "number": true, "single_select": true, "multi_select": true, "datetime": true, "static_text": true, "product": true}
	for _, f := range schema.Fields {
		if strings.TrimSpace(f.Key) == "" {
			return errors.New("字段 key 不能为空")
		}
		if seen[f.Key] {
			return fmt.Errorf("字段 key 重复: %s", f.Key)
		}
		seen[f.Key] = true
		if !allowed[f.Type] {
			return fmt.Errorf("不支持的字段类型: %s", f.Type)
		}
		if (f.Type == "single_select" || f.Type == "multi_select") && len(f.Options) == 0 {
			return fmt.Errorf("选择字段 %s 必须配置选项", f.Key)
		}
		if f.Type == "number" && f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			return fmt.Errorf("字段 %s 的最小值不能大于最大值", f.Key)
		}
		if f.Type == "product" && (f.AllowMultiple || f.Repeatable) {
			return fmt.Errorf("产品字段 %s 不支持多个测量值或登记表明细", f.Key)
		}
	}
	return nil
}

// ResolveDefault 将双花括号模板变量替换为当前业务上下文。
func ResolveDefault(value string, ctx DefaultContext) string {
	if value == "" {
		return ""
	}
	return tokenPattern.ReplaceAllStringFunc(value, func(token string) string {
		match := tokenPattern.FindStringSubmatch(token)
		if len(match) != 2 {
			return ""
		}
		switch match[1] {
		case "current_date":
			return ctx.Date
		case "current_time":
			return ctx.Time
		case "current_shift":
			return ctx.Shift
		case "user_name":
			return ctx.UserName
		case "user_account":
			return ctx.UserAccount
		case "line_name":
			return ctx.LineName
		case "line_product":
			return ctx.LineProduct
		case "form_name":
			return ctx.FormName
		case "stage":
			return ctx.Stage
		default:
			return ""
		}
	})
}

func FieldMap(schema FormSchema) map[string]FieldDefinition {
	out := make(map[string]FieldDefinition, len(schema.Fields))
	for _, f := range schema.Fields {
		out[f.Key] = f
	}
	return out
}

func VersionLabel(kind string) string {
	switch kind {
	case model.FormKindFirstInspection:
		return "首检"
	case model.FormKindFinalInspection:
		return "末检"
	case model.FormKindReport:
		return "报表"
	case model.FormKindRegistration:
		return "登记表"
	default:
		return kind
	}
}
func HasProductField(schema FormSchema) bool {
	for _, f := range schema.Fields {
		if f.Type == "product" {
			return true
		}
	}
	return false
}

// PrependProductField 为旧首末检模板补充产品字段并生成新草稿版本。
func PrependProductField(schema FormSchema) FormSchema {
	used := make(map[string]bool, len(schema.Fields))
	for _, f := range schema.Fields {
		used[f.Key] = true
	}
	key := "product"
	for i := 2; used[key]; i++ {
		key = fmt.Sprintf("product_%d", i)
	}
	fields := make([]FieldDefinition, 0, len(schema.Fields)+1)
	fields = append(fields, FieldDefinition{Key: key, Label: "产品", Type: "product", Required: true, SortOrder: 0})
	for _, f := range schema.Fields {
		f.SortOrder++
		fields = append(fields, f)
	}
	return FormSchema{Fields: fields}
}
