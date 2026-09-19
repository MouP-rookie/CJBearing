// 文件说明：数字字段上下限判断辅助函数。

package httpapi

import "bearing-inspection/internal/service"

// isValueOutOfRange 只对数值字段判断上下限，超出范围仅做提示不阻断提交。
func isValueOutOfRange(value *float64, field service.FieldDefinition) bool {
	if value == nil || field.Type != "number" {
		return false
	}
	return (field.Min != nil && *value < *field.Min) || (field.Max != nil && *value > *field.Max)
}
