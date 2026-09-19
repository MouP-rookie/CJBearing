package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"

	"bearing-inspection/internal/model"
)

func TestParseDepartmentArchive(t *testing.T) {
	manifest, _ := json.Marshal(archiveManifest{Version: 1, DepartmentID: 7, Department: "测试部门"})
	data, _ := json.Marshal(departmentArchive{Department: model.Department{ID: 7, Code: "test", Name: "测试部门", Enabled: true}})
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, payload := range map[string][]byte{"manifest.json": manifest, "data.json": data} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	gotManifest, gotData, err := parseDepartmentArchive(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if gotManifest.Version != 1 || gotData.Department.Code != "test" {
		t.Fatalf("unexpected archive: %+v %+v", gotManifest, gotData.Department)
	}
}
