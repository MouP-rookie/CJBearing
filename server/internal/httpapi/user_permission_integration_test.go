package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/database"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"bearing-inspection/internal/service/backup"
	"bearing-inspection/internal/service/instrument"
)

func TestSuperAdminCanChangeUserRoleInAllDepartmentMode(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := database.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	var superRole, adminRole, normalRole model.Role
	if err := db.Where("code = ?", "super_admin").First(&superRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("code = ?", "admin").First(&adminRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("code = ?", "normal").First(&normalRole).Error; err != nil {
		t.Fatal(err)
	}
	var superUser model.User
	if err := db.Where("role_id = ?", superRole.ID).First(&superUser).Error; err != nil {
		t.Fatal(err)
	}
	department := model.Department{Code: fmt.Sprintf("permission_test_%d", time.Now().UnixNano()), Name: "权限测试部门", Enabled: true}
	if err := db.Create(&department).Error; err != nil {
		t.Fatal(err)
	}
	user := model.User{Account: department.Code, Name: "权限测试用户", PasswordHash: "test", DepartmentID: &department.ID, RoleID: normalRole.ID, Enabled: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Where("entity_type = ? AND entity_id = ?", "user", user.ID).Delete(&model.AuditLog{})
		db.Unscoped().Delete(&user)
		db.Unscoped().Delete(&department)
	})

	authService := auth.NewService("permission-test-secret", time.Hour)
	token, _, err := authService.GenerateToken(superUser.ID, superUser.Account, auth.ClientPC)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(&config.Config{AppEnv: "test"}, db, authService, rbac.NewAuthorizer(db), backup.New(db, "", t.TempDir()), instrument.NewDBStore(db))
	body, _ := json.Marshal(map[string]any{"roleId": adminRole.ID})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+strconv.FormatUint(user.ID, 10), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var updated model.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.RoleID != adminRole.ID {
		t.Fatalf("expected role %d, got %d", adminRole.ID, updated.RoleID)
	}
}
