package qms

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDeviceAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "device-aliases.json")
	content := `{
  "aliases": [
    {
      "deviceId": "device-new",
      "legacyDeviceId": "device-old",
      "switchAt": "2026-11-02T13:33:05.787+08:00"
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write aliases: %v", err)
	}
	aliases, err := LoadDeviceAliases(path)
	if err != nil {
		t.Fatalf("load aliases: %v", err)
	}
	segments := aliases["device-new"]
	if len(segments) != 1 {
		t.Fatalf("expected one segment, got %#v", aliases)
	}
	if segments[0].LegacyDeviceID != "device-old" {
		t.Fatalf("unexpected legacy device: %#v", segments[0])
	}
	if segments[0].SwitchTs != 1793597585787 {
		t.Fatalf("unexpected switch timestamp: %d", segments[0].SwitchTs)
	}
}

func TestLoadDeviceAliasesMissingFile(t *testing.T) {
	aliases, err := LoadDeviceAliases(filepath.Join(t.TempDir(), "not-exists.json"))
	if err != nil {
		t.Fatalf("missing alias file must not fail startup: %v", err)
	}
	if len(aliases) != 0 {
		t.Fatalf("expected empty aliases, got %#v", aliases)
	}
}

func TestLegacySegmentsSkipRangeBeforeSwitch(t *testing.T) {
	service := &HistoryService{aliases: map[string][]legacyAlias{
		"device-new": {{DeviceID: "device-new", LegacyDeviceID: "device-old", SwitchTs: 2000}},
	}}
	if got := service.legacySegments("device-new", 2000, 3000); len(got) != 0 {
		t.Fatalf("range starting at switch time must not include legacy data: %#v", got)
	}
	if got := service.legacySegments("device-new", 1000, 3000); len(got) != 1 {
		t.Fatalf("range crossing switch time must include legacy data: %#v", got)
	}
	if got := service.legacySegments("other-device", 1000, 3000); len(got) != 0 {
		t.Fatalf("unrelated device must not include legacy data: %#v", got)
	}
}

func TestMergeSeriesKeepsCurrentValues(t *testing.T) {
	base := map[string]map[int64]string{
		"角游隙测量值": {3000: "50"},
	}
	legacy := map[string]map[int64]string{
		"角游隙测量值": {1000: "10", 3000: "99"},
		"角游隙上限":  {1000: "100"},
	}
	merged := mergeSeries(base, legacy, 2500)
	if merged["角游隙测量值"][3000] != "50" {
		t.Fatalf("current device value must win at same timestamp: %#v", merged["角游隙测量值"])
	}
	if merged["角游隙测量值"][1000] != "10" {
		t.Fatalf("legacy value before switch must be merged: %#v", merged["角游隙测量值"])
	}
	if merged["角游隙上限"][1000] != "100" {
		t.Fatalf("legacy-only field must be merged: %#v", merged["角游隙上限"])
	}
}

func TestMergeRawPayloadsFiltersAndOrders(t *testing.T) {
	current := json.RawMessage(`{"角游隙测量值":[{"ts":3000,"value":"50"},{"ts":4000,"value":"60"}]}`)
	legacy := json.RawMessage(`{"角游隙测量值":[{"ts":1000,"value":"10"},{"ts":2000,"value":"20"},{"ts":3500,"value":"99"}],"workNum":[{"ts":1500,"value":"7"}]}`)
	merged, err := mergeRawPayloads(
		[]rawSource{{Payload: current}, {Payload: legacy, Until: 2500}},
		[]string{"角游隙测量值", "workNum", "状态"},
		2,
		true,
	)
	if err != nil {
		t.Fatalf("merge raw payloads: %v", err)
	}
	var parsed map[string][]rawPoint
	if err := json.Unmarshal(merged, &parsed); err != nil {
		t.Fatalf("unmarshal merged payload: %v", err)
	}
	points := parsed["角游隙测量值"]
	if len(points) != 2 {
		t.Fatalf("expected limit to keep two points, got %#v", points)
	}
	if points[0].TS != 4000 || points[1].TS != 3000 {
		t.Fatalf("expected descending order after merge, got %#v", points)
	}
	if len(parsed["workNum"]) != 1 || parsed["workNum"][0].TS != 1500 {
		t.Fatalf("legacy-only field must be merged: %#v", parsed["workNum"])
	}
	if _, ok := parsed["状态"]; !ok {
		t.Fatalf("requested field without data must stay present as empty array")
	}
}

func TestUnionKeys(t *testing.T) {
	got := unionKeys([]string{"a", "b"}, []string{"b", "c"}, nil)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("unexpected keys: %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected keys: %#v", got)
		}
	}
}
