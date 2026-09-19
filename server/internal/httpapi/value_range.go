package httpapi

import "bearing-inspection/internal/service"

func isValueOutOfRange(value *float64, field service.FieldDefinition) bool {
	if value == nil || field.Type != "number" {
		return false
	}
	return (field.Min != nil && *value < *field.Min) || (field.Max != nil && *value > *field.Max)
}
