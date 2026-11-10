package qms

import "testing"

func TestDetectFieldGroupsAndEvaluate(t *testing.T) {
	groups := detectFieldGroups([]string{
		"温度测量值", "温度上限", "温度下限", "code", "status", "spcvalue",
	})
	if len(groups) != 3 {
		t.Fatalf("expected temperature plus two plain fields, got %#v", groups)
	}
	group := groups[0]
	if group.Name != "温度" || !group.HasLimit || group.Source != "field" {
		t.Fatalf("unexpected temperature group: %#v", group)
	}
	if flag := evaluateGroup(group, map[string]string{
		"温度测量值": "18.5", "温度上限": "20", "温度下限": "10",
	}); flag != "OK" {
		t.Fatalf("expected OK, got %q", flag)
	}
	if flag := evaluateGroup(group, map[string]string{
		"温度测量值": "21", "温度上限": "20", "温度下限": "10",
	}); flag != "OUT" {
		t.Fatalf("expected OUT, got %q", flag)
	}
	if flag := evaluateGroup(group, map[string]string{
		"温度测量值": "0", "温度上限": "0", "温度下限": "0",
	}); flag != "" {
		t.Fatalf("zero placeholder limits must not be judgeable, got %q", flag)
	}
}

func TestBooleanAndSPCJudgement(t *testing.T) {
	booleanGroup := HistoryGroup{Name: "外观OK/NG", ValueKey: "外观测量值"}
	if flag := evaluateGroup(booleanGroup, map[string]string{"外观测量值": "0"}); flag != "OUT" {
		t.Fatalf("0 must be NG, got %q", flag)
	}
	if flag := evaluateGroup(booleanGroup, map[string]string{"外观测量值": "1"}); flag != "OK" {
		t.Fatalf("1 must be OK, got %q", flag)
	}

	service := &HistoryService{}
	rows, truncated := service.spcRows(map[int64]string{
		1000: `[{"Name":"注脂量","Ucl":"7","Lcl":"6","Value":"6.5"}]`,
		2000: `[{"Name":"注脂量","Ucl":"7","Lcl":"6","Value":"7.5"}]`,
	}, true)
	if truncated || len(rows) != 2 {
		t.Fatalf("unexpected SPC rows: %#v truncated=%v", rows, truncated)
	}
	if !rows[0].Out || !rows[0].Applicable {
		t.Fatalf("newest row should be out of limits: %#v", rows[0])
	}
	if rows[1].Out {
		t.Fatalf("older row should be within limits: %#v", rows[1])
	}
}
