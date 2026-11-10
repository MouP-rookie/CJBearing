package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/config"
	"bearing-inspection/internal/database"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/qms"
	"bearing-inspection/internal/rbac"
	"github.com/google/uuid"
)

// 使用回滚事务创建独立账号，验证一次门户密码登录贯穿多个服务和端口。
func TestPortalSingleSignOn(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := database.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	code := "sso_" + uuid.NewString()
	department := model.Department{Code: code, Name: "统一登录测试", Enabled: true}
	check(tx.Create(&department).Error)
	role := model.Role{Code: code, Name: "统一登录测试角色", AllowPC: true, AllowPDA: true}
	check(tx.Create(&role).Error)
	var pcPermissionID uint64
	for _, permissionCode := range []string{rbac.PermPCLogin, rbac.PermPDALogin, rbac.PermLineManage, rbac.PermExecutionView, rbac.PermQMSView} {
		check(tx.Exec(`INSERT INTO permissions (code, name) VALUES (?, ?) ON CONFLICT (code) DO NOTHING`, permissionCode, permissionCode).Error)
		var permission model.Permission
		check(tx.Where("code = ?", permissionCode).First(&permission).Error)
		check(tx.Create(&model.RolePermission{RoleID: role.ID, PermissionID: permission.ID}).Error)
		if permissionCode == rbac.PermPCLogin {
			pcPermissionID = permission.ID
		}
	}
	hash, err := auth.HashPassword("test-sso-password")
	check(err)
	user := model.User{Account: code, Name: "统一身份用户", PasswordHash: hash, DepartmentID: &department.ID, RoleID: role.ID, Enabled: true}
	check(tx.Create(&user).Error)
	for _, system := range []string{"bearing_inspection", "qms_iot"} {
		check(tx.Exec(`INSERT INTO portal_systems (code, name, entry_path, entry_port) VALUES (?, ?, '/', 8090) ON CONFLICT (code) DO NOTHING`, system, system).Error)
		check(tx.Exec(`INSERT INTO portal_system_access (user_id, system_code, enabled) VALUES (?, ?, TRUE)`, user.ID, system).Error)
	}
	staticDir := t.TempDir()
	check(os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("shared-session-qms"), 0600))
	cfg := &config.Config{AppEnv: "test", JWTTTL: time.Hour, QMSStaticDir: staticDir}
	service := auth.NewService("sso-integration-test-secret", time.Hour)
	authorizer := rbac.NewAuthorizer(tx)
	apiServer := httptest.NewServer(NewServer(cfg, tx, service, authorizer, nil, nil).Router())
	t.Cleanup(apiServer.Close)
	qmsServer := httptest.NewServer(qms.NewServer(cfg, tx, service, authorizer).Router())
	t.Cleanup(qmsServer.Close)
	jar, err := cookiejar.New(nil)
	check(err)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(method, address, body string, want int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, address, strings.NewReader(body))
		check(err)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := client.Do(req)
		check(err)
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		check(err)
		if response.StatusCode != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, address, response.StatusCode, want, data)
		}
		return data
	}
	loginBody := `{"account":"` + user.Account + `","password":"test-sso-password"}`
	request(http.MethodPost, apiServer.URL+"/api/v1/auth/portal/login", loginBody, http.StatusOK)
	for _, clientName := range []string{"pc", "pda"} {
		data := request(http.MethodGet, apiServer.URL+"/api/v1/me?client="+clientName, "", http.StatusOK)
		var result struct {
			User   struct{ ID uint64 } `json:"user"`
			Client string              `json:"client"`
		}
		check(json.Unmarshal(data, &result))
		if result.User.ID != user.ID || result.Client != auth.ClientPortal {
			t.Fatalf("subsystem changed identity or login source: %s", data)
		}
	}
	request(http.MethodGet, apiServer.URL+"/api/v1/admin/lines", "", http.StatusOK)
	if data := request(http.MethodGet, qmsServer.URL+"/", "", http.StatusOK); string(data) != "shared-session-qms" {
		t.Fatalf("unexpected QMS page: %s", data)
	}
	request(http.MethodGet, apiServer.URL+"/api/v1/portal/admin/users", "", http.StatusForbidden)

	t.Log("role permission revocation applies to existing portal login")
	{
		check(tx.Where("role_id = ? AND permission_id = ?", role.ID, pcPermissionID).Delete(&model.RolePermission{}).Error)
		request(http.MethodGet, apiServer.URL+"/api/v1/me?client=pc", "", http.StatusForbidden)
		request(http.MethodGet, apiServer.URL+"/api/v1/admin/lines", "", http.StatusForbidden)
		check(tx.Create(&model.RolePermission{RoleID: role.ID, PermissionID: pcPermissionID}).Error)
	}
	t.Log("system access is still enforced")
	{
		check(tx.Exec(`UPDATE portal_system_access SET enabled = FALSE WHERE user_id = ? AND system_code = 'bearing_inspection'`, user.ID).Error)
		request(http.MethodGet, apiServer.URL+"/api/v1/me?client=pc", "", http.StatusForbidden)
		request(http.MethodGet, qmsServer.URL+"/", "", http.StatusOK)
		check(tx.Exec(`UPDATE portal_system_access SET enabled = TRUE WHERE user_id = ?`, user.ID).Error)
	}
	t.Log("disabled account cannot reuse shared session")
	{
		check(tx.Model(&model.User{}).Where("id = ?", user.ID).Update("enabled", false).Error)
		request(http.MethodGet, apiServer.URL+"/api/v1/me?client=pc", "", http.StatusUnauthorized)
		request(http.MethodGet, qmsServer.URL+"/", "", http.StatusFound)
		check(tx.Model(&model.User{}).Where("id = ?", user.ID).Update("enabled", true).Error)
	}
	t.Log("PDA number login cannot escalate into PC")
	{
		request(http.MethodPost, apiServer.URL+"/api/v1/auth/pda/login", `{"account":"`+user.Account+`"}`, http.StatusOK)
		request(http.MethodGet, apiServer.URL+"/api/v1/admin/lines", "", http.StatusForbidden)
		request(http.MethodGet, apiServer.URL+"/api/v1/me?client=pc", "", http.StatusForbidden)
		request(http.MethodGet, apiServer.URL+"/api/v1/me?client=pda", "", http.StatusOK)
	}
	t.Log("logout clears the cookie across service ports")
	{
		request(http.MethodPost, apiServer.URL+"/api/v1/auth/portal/login", loginBody, http.StatusOK)
		request(http.MethodGet, qmsServer.URL+"/", "", http.StatusOK)
		request(http.MethodPost, apiServer.URL+"/api/v1/auth/logout", "", http.StatusNoContent)
		request(http.MethodGet, apiServer.URL+"/api/v1/me?client=pc", "", http.StatusUnauthorized)
		request(http.MethodGet, qmsServer.URL+"/", "", http.StatusFound)
	}
}
