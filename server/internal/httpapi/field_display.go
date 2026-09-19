// 文件说明：字段值显示转换，将选项 value 转为界面 label。

package httpapi

import (
	"strings"

	"bearing-inspection/internal/service"
)

// displayFieldText 将单多选字段存储值转换为配置的中文标签。
func displayFieldText(value string, field service.FieldDefinition) string {
	if field.Type != "single_select" && field.Type != "multi_select" {
		return value
	}
	for _, option := range field.Options {
		if option.Value == value {
			if label := strings.TrimSpace(option.Label); label != "" {
				return label
			}
			return value
		}
	}
	return value
}
