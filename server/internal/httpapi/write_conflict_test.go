package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTemplateCodeConflict(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		constraint string
		want       bool
	}{
		{"same department code", "23505", "ux_form_templates_department_code_active", true},
		{"unrelated unique violation", "23505", "form_versions_template_id_version_no_key", false},
		{"different SQL state", "23503", "ux_form_templates_department_code_active", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			err := fmt.Errorf("transaction: %w", &pgconn.PgError{Code: tc.state, ConstraintName: tc.constraint, Detail: "internal database detail"})
			if got := respondTemplateCodeConflict(c, err, "FIRST_LINE_01"); got != tc.want {
				t.Fatalf("handled = %t, want %t", got, tc.want)
			}
			body := recorder.Body.String()
			if tc.want && (recorder.Code != http.StatusConflict || !strings.Contains(body, "template_code_conflict") || !strings.Contains(body, "当前部门已使用模板编码「FIRST_LINE_01」") || strings.Contains(body, "internal database detail")) {
				t.Fatalf("unexpected response: %d %s", recorder.Code, body)
			}
			if !tc.want && body != "" {
				t.Fatalf("unexpected conflict response: %s", body)
			}
		})
	}
}
