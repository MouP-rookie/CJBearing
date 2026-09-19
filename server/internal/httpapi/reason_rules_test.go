package httpapi

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeVoidReasons(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr string
	}{
		{name: "normalizes values", input: []string{" 录入错误 ", "重复提交"}, want: []string{"录入错误", "重复提交"}},
		{name: "allows empty list", input: nil, want: []string{}},
		{name: "rejects empty value", input: []string{""}, wantErr: "不能为空"},
		{name: "rejects reserved option", input: []string{"其他"}, wantErr: "系统保留"},
		{name: "rejects duplicate", input: []string{"录入错误", " 录入错误 "}, wantErr: "重复"},
		{name: "rejects too long", input: []string{strings.Repeat("原", 51)}, wantErr: "不能超过"},
		{name: "rejects too many", input: make([]string, 51), wantErr: "最多配置"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeVoidReasons(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("normalizeVoidReasons() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeVoidReasons() unexpected error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeVoidReasons() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizeActionReason(t *testing.T) {
	got, err := normalizeActionReason("  录入错误  ", "废除")
	if err != nil || got != "录入错误" {
		t.Fatalf("normalizeActionReason() = %q, %v", got, err)
	}
	if _, err := normalizeActionReason(" ", "恢复"); err == nil {
		t.Fatal("normalizeActionReason() should reject blank reason")
	}
	if _, err := normalizeActionReason(strings.Repeat("原", 501), "废除"); err == nil {
		t.Fatal("normalizeActionReason() should reject overlong reason")
	}
}
