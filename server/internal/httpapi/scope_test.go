package httpapi

import (
	"strings"
	"testing"

	"bearing-inspection/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDepartmentScopeCanAccessDepartment(t *testing.T) {
	selected := &departmentScope{DepartmentID: 7}
	if !selected.CanAccessDepartment(7) {
		t.Fatal("selected department should be accessible")
	}
	if selected.CanAccessDepartment(8) {
		t.Fatal("other department must not be accessible")
	}
	all := &departmentScope{AllDepartments: true}
	if !all.CanAccessDepartment(8) {
		t.Fatal("super-admin all-department scope should access every department")
	}
}

func TestDepartmentScopeFilterAddsDepartmentPredicate(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	var lines []model.Line
	tx := (&departmentScope{DepartmentID: 7}).Filter(db.Model(&model.Line{}), "department_id").Find(&lines)
	if !strings.Contains(tx.Statement.SQL.String(), "department_id") {
		t.Fatalf("expected department predicate in SQL, got %s", tx.Statement.SQL.String())
	}
}

func TestCompareLineCodeUsesNaturalNumericOrder(t *testing.T) {
	ordered := []string{"装配2线", "装配3线", "装配10线", "装配20线", "装配100线"}
	for index := 1; index < len(ordered); index++ {
		if compareLineCode(ordered[index-1], ordered[index]) >= 0 {
			t.Fatalf("expected %q before %q", ordered[index-1], ordered[index])
		}
	}
}
