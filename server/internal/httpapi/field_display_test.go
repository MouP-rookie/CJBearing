package httpapi

import (
	"testing"

	"bearing-inspection/internal/service"
)

func TestDisplayFieldText(t *testing.T) {
	field := service.FieldDefinition{
		Type: "single_select",
		Options: []service.FieldOption{
			{Label: "合格", Value: "option_1"},
			{Label: "不合格", Value: "option_2"},
		},
	}

	tests := []struct {
		name  string
		value string
		field service.FieldDefinition
		want  string
	}{
		{name: "configured option label", value: "option_1", field: field, want: "合格"},
		{name: "unknown option value", value: "option_3", field: field, want: "option_3"},
		{name: "non-select value", value: "option_1", field: service.FieldDefinition{Type: "text", Options: field.Options}, want: "option_1"},
		{name: "blank configured label", value: "option_1", field: service.FieldDefinition{Type: "single_select", Options: []service.FieldOption{{Label: "  ", Value: "option_1"}}}, want: "option_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayFieldText(tt.value, tt.field); got != tt.want {
				t.Fatalf("displayFieldText() = %q, want %q", got, tt.want)
			}
		})
	}
}
