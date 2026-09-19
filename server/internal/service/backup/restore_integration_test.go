package backup

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestValidateCurrentDepartmentArchive(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	dir := os.Getenv("BACKUP_DIR")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "backups")
	}
	files, err := filepath.Glob(filepath.Join(dir, "dept_*.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no department backup archives found")
	}
	sort.Strings(files)
	payload, err := os.ReadFile(files[len(files)-1])
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(db, "", dir).ValidateDepartmentArchive(payload); err != nil {
		t.Fatalf("current archive failed transactional validation: %v", err)
	}
}
