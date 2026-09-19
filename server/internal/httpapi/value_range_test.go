package httpapi

import (
	"testing"

	"bearing-inspection/internal/service"
)

func TestIsValueOutOfRange(t *testing.T) {
	min := 10.0
	max := 20.0
	below := 9.99
	atMin := 10.0
	atMax := 20.0
	above := 20.01

	tests := []struct {
		name  string
		value *float64
		field service.FieldDefinition
		want  bool
	}{
		{name: "missing value", field: service.FieldDefinition{Type: "number", Min: &min, Max: &max}, want: false},
		{name: "non-number field", value: &above, field: service.FieldDefinition{Type: "text", Min: &min, Max: &max}, want: false},
		{name: "no limits", value: &above, field: service.FieldDefinition{Type: "number"}, want: false},
		{name: "below lower limit", value: &below, field: service.FieldDefinition{Type: "number", Min: &min, Max: &max}, want: true},
		{name: "at lower limit", value: &atMin, field: service.FieldDefinition{Type: "number", Min: &min, Max: &max}, want: false},
		{name: "at upper limit", value: &atMax, field: service.FieldDefinition{Type: "number", Min: &min, Max: &max}, want: false},
		{name: "above upper limit", value: &above, field: service.FieldDefinition{Type: "number", Min: &min, Max: &max}, want: true},
		{name: "only lower limit", value: &below, field: service.FieldDefinition{Type: "number", Min: &min}, want: true},
		{name: "only upper limit", value: &above, field: service.FieldDefinition{Type: "number", Max: &max}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValueOutOfRange(tt.value, tt.field); got != tt.want {
				t.Fatalf("isValueOutOfRange() = %v, want %v", got, tt.want)
			}
		})
	}
}
