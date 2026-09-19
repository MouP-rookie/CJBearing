package httpapi

import "testing"

func TestLineImportRoutesRegistered(t *testing.T) {
	server := NewServer(nil, nil, nil, nil, nil, nil)
	routes := map[string]bool{}
	for _, route := range server.Router().Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{"GET /api/v1/admin/lines/import-template", "POST /api/v1/admin/lines/import", "POST /api/v1/admin/backups/restore"} {
		if !routes[route] {
			t.Fatalf("route %s is not registered", route)
		}
	}
}

func TestParseLineImportHeader(t *testing.T) {
	columns, err := parseLineImportHeader([]string{"产线编码", "产线名称", "上级产线编码", "状态"})
	if err != nil {
		t.Fatal(err)
	}
	if columns.code != 0 || columns.name != 1 || columns.parent != 2 || columns.status != 3 {
		t.Fatalf("unexpected columns: %+v", columns)
	}
}

func TestParseLineImportEnabled(t *testing.T) {
	cases := map[string]bool{"": true, "启用": true, "是": true, "true": true, "停用": false, "否": false, "false": false}
	for input, want := range cases {
		got, err := parseLineImportEnabled(input)
		if err != nil {
			t.Fatalf("input %q: %v", input, err)
		}
		if got != want {
			t.Fatalf("input %q: want %v, got %v", input, want, got)
		}
	}
	if _, err := parseLineImportEnabled("未知"); err == nil {
		t.Fatal("expected invalid status error")
	}
}
