package httpapi

import (
	"net/http"
	"strings"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
)

type pcLoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type pdaLoginRequest struct {
	Account string `json:"account" binding:"required"`
}

func (s *Server) pcLogin(c *gin.Context) {
	var req pcLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_request", "请输入账号和密码", err.Error())
		return
	}
	user, err := s.findUserByAccount(strings.TrimSpace(req.Account))
	if err != nil || !user.Enabled || user.Role.DeletedAt.Valid || !user.Role.AllowPC || !s.userDepartmentEnabled(user) {
		response.Fail(c, http.StatusUnauthorized, "login_failed", "账号或密码错误", nil)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		response.Fail(c, http.StatusUnauthorized, "login_failed", "账号或密码错误", nil)
		return
	}
	if ok, _ := s.authorizer.Can(user.ID, rbac.PermPCLogin, nil); !ok {
		response.Fail(c, http.StatusForbidden, "forbidden", "该账号不能登录PC", nil)
		return
	}
	s.issueToken(c, user, auth.ClientPC)
}

func (s *Server) pdaLogin(c *gin.Context) {
	var req pdaLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_request", "请输入工号", err.Error())
		return
	}
	user, err := s.findUserByAccount(strings.TrimSpace(req.Account))
	if err != nil || !user.Enabled || user.Role.DeletedAt.Valid || !user.Role.AllowPDA || !s.userDepartmentEnabled(user) {
		response.Fail(c, http.StatusUnauthorized, "login_failed", "账号不存在或已停用", nil)
		return
	}
	if ok, _ := s.authorizer.Can(user.ID, rbac.PermPDALogin, nil); !ok {
		response.Fail(c, http.StatusForbidden, "forbidden", "该账号不能登录PDA", nil)
		return
	}
	s.issueToken(c, user, auth.ClientPDA)
}

func (s *Server) issueToken(c *gin.Context, user *model.User, client string) {
	token, expiresAt, err := s.auth.GenerateToken(user.ID, user.Account, client)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "token_error", "生成登录凭证失败", nil)
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, int(s.cfg.JWTTTL.Seconds()), "/", "", s.cfg.AppEnv == "production", true)
	response.OK(c, gin.H{"token": token, "expiresAt": expiresAt, "user": publicUser(user)})
}

func (s *Server) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", s.cfg.AppEnv == "production", true)
	response.NoContent(c)
}

func (s *Server) me(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	user := scope.User
	perms, _ := s.userPermissions(user)
	lines, _ := s.authorizer.AllowedLines(user.ID, scope.DepartmentID)
	var departments []model.Department
	if user.Role.Code == "super_admin" {
		_ = s.db.Order("code, id").Find(&departments).Error
	}
	logo, _ := s.readBrandLogo(scope.DepartmentID)
	response.OK(c, gin.H{
		"user": publicUser(user), "permissions": perms, "lineIds": lines,
		"client": c.GetString("client"), "isSuperAdmin": user.Role.Code == "super_admin",
		"departments": departments, "selectedDepartmentId": scope.DepartmentID,
		"brandLogoDataUrl": logo,
	})
}

func (s *Server) userDepartmentEnabled(user *model.User) bool {
	if user.Role.Code == "super_admin" {
		return true
	}
	if user.DepartmentID == nil || user.Department == nil || !user.Department.Enabled || user.Department.DeletedAt.Valid {
		return false
	}
	return true
}

func (s *Server) findUserByAccount(account string) (*model.User, error) {
	var user model.User
	err := s.db.Preload("Role").Preload("Department").Where("account = ?", account).First(&user).Error
	return &user, err
}

func (s *Server) userPermissions(user *model.User) ([]string, error) {
	if user.Role.Code == "super_admin" {
		return rbac.AllPermissionCodes(), nil
	}
	var codes []string
	err := s.db.Table("permissions p").Select("p.code").
		Joins("JOIN role_permissions rp ON rp.permission_id = p.id").
		Where("rp.role_id = ?", user.RoleID).Order("p.code").Pluck("p.code", &codes).Error
	return codes, err
}

func publicUser(user *model.User) gin.H {
	return gin.H{"id": user.ID, "account": user.Account, "name": user.Name, "enabled": user.Enabled, "role": user.Role, "departmentId": user.DepartmentID, "department": user.Department}
}
