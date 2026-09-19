package httpapi

import (
	"strings"

	"bearing-inspection/internal/service"
)

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
