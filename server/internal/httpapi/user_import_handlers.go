// 文件说明：用户 Excel 模板、批量导入、角色部门解析和行级校验。

package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"bearing-inspection/internal/auth"
	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const userImportMaxBytes = 10 << 20

type userImportRowError struct {
	Row     int    `json:"row"`
	Account string `json:"account"`
	Message string `json:"message"`
}

type userImportColumns struct {
	account    int
	name       int
	role       int
	department int
}

// userImportTemplate 提供用户批量导入 Excel 模板。
func (s *Server) userImportTemplate(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	f := excelize.NewFile()
	sheet := "用户导入"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		response.Fail(c, 500, "template_failed", "生成导入模板失败", err.Error())
		return
	}
	if scope.User.Role.Code == "super_admin" {
		_ = f.SetSheetRow(sheet, "A1", &[]any{"工号", "用户名", "角色", "部门"})
		_ = f.SetSheetRow(sheet, "A2", &[]any{"10002", "张三", "普通用户", "东港分厂"})
		_ = f.SetColWidth(sheet, "A", "D", 20)
	} else {
		_ = f.SetSheetRow(sheet, "A1", &[]any{"工号", "用户名", "角色"})
		_ = f.SetSheetRow(sheet, "A2", &[]any{"10002", "张三", "普通用户"})
		_ = f.SetColWidth(sheet, "A", "C", 20)
	}
	writeWorkbook(c, f, "用户批量导入模板.xlsx")
}

// importUsers 校验部门、角色和重复工号后批量创建用户。
func (s *Server) importUsers(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if scope.AllDepartments {
		response.Fail(c, http.StatusBadRequest, "department_required", "请先选择部门", nil)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, userImportMaxBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, 400, "invalid_file", "请选择 Excel 文件", err.Error())
		return
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".xlsx") {
		response.Fail(c, 400, "invalid_file", "只支持 .xlsx 格式的 Excel 文件", nil)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, 400, "invalid_file", "无法读取 Excel 文件", err.Error())
		return
	}
	defer file.Close()
	workbook, err := excelize.OpenReader(file)
	if err != nil {
		response.Fail(c, 400, "invalid_file", "Excel 文件格式不正确", err.Error())
		return
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		response.Fail(c, 400, "invalid_file", "Excel 中没有可读取的工作表", nil)
		return
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		response.Fail(c, 400, "invalid_file", "读取 Excel 内容失败", err.Error())
		return
	}
	if len(rows) == 0 {
		response.Fail(c, 400, "invalid_file", "Excel 中没有数据", nil)
		return
	}
	columns, err := parseUserImportHeader(rows[0])
	if err != nil {
		response.Fail(c, 400, "invalid_header", err.Error(), nil)
		return
	}
	current := scope.User
	normalRole, err := s.roleByCode("normal")
	if err != nil {
		response.Fail(c, 500, "seed_error", "系统普通用户角色不存在", err.Error())
		return
	}
	passwordHash, err := auth.HashPassword("123456")
	if err != nil {
		response.Fail(c, 500, "hash_error", "生成默认密码失败", err.Error())
		return
	}

	created := 0
	rowErrors := make([]userImportRowError, 0)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if isBlankUserImportRow(row) {
			continue
		}
		rowNo := i + 1
		account := userImportCell(row, columns.account)
		name := userImportCell(row, columns.name)
		roleValue := userImportCell(row, columns.role)
		departmentValue := userImportCell(row, columns.department)
		if account == "" {
			rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "工号不能为空"})
			continue
		}
		if name == "" {
			rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "用户名不能为空"})
			continue
		}
		role := normalRole
		if roleValue != "" {
			resolved, roleErr := s.importRole(roleValue)
			if roleErr != nil {
				rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "角色不存在：" + roleValue})
				continue
			}
			role = resolved
		}
		if role.Code == "super_admin" {
			rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "系统只允许一个超级管理员，不能导入超级管理员"})
			continue
		}
		var requestedDepartmentID *uint64
		if current.Role.Code == "super_admin" && role.Code != "super_admin" {
			if departmentValue != "" {
				department, departmentErr := s.departmentForImport(departmentValue)
				if departmentErr != nil {
					rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "部门不存在：" + departmentValue})
					continue
				}
				requestedDepartmentID = &department.ID
			}
		}
		departmentID, departmentErr := s.resolveUserDepartment(scope, requestedDepartmentID, role.Code)
		if departmentErr != nil {
			rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: departmentErr.Error()})
			continue
		}
		var existing model.User
		if err := s.db.Where("account = ?", account).First(&existing).Error; err == nil {
			rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "账号已存在"})
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "检查账号失败"})
			continue
		}
		user := model.User{Account: account, Name: name, PasswordHash: passwordHash, DepartmentID: departmentID, RoleID: role.ID, Enabled: true}
		if err := s.db.Create(&user).Error; err != nil {
			rowErrors = append(rowErrors, userImportRowError{Row: rowNo, Account: account, Message: "创建失败：" + err.Error()})
			continue
		}
		s.audit(c, "user", user.ID, "import", nil, user)
		created++
	}
	response.OK(c, gin.H{
		"created":         created,
		"failed":          len(rowErrors),
		"errors":          rowErrors,
		"defaultPassword": "123456",
	})
}

func (s *Server) roleByCode(code string) (*model.Role, error) {
	var role model.Role
	return &role, s.db.Where("code = ?", code).First(&role).Error
}

// importRole 将导入单元格中的角色名称或编码解析为角色记录。
func (s *Server) importRole(value string) (*model.Role, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return s.roleByCode("normal")
	}
	if id, err := strconv.ParseUint(value, 10, 64); err == nil {
		return s.roleByID(id)
	}
	var role model.Role
	err := s.db.Where("code = ? OR name = ?", value, value).First(&role).Error
	return &role, err
}

func (s *Server) departmentForImport(value string) (*model.Department, error) {
	value = strings.TrimSpace(value)
	var department model.Department
	if id, err := strconv.ParseUint(value, 10, 64); err == nil {
		return &department, s.db.Where("id = ? AND enabled = ?", id, true).First(&department).Error
	}
	err := s.db.Where("enabled = ? AND (code = ? OR name = ?)", true, value, value).First(&department).Error
	return &department, err
}

func parseUserImportHeader(row []string) (userImportColumns, error) {
	columns := userImportColumns{account: -1, name: -1, role: -1, department: -1}
	for i, value := range row {
		switch normalizeUserImportHeader(value) {
		case "工号", "账号", "用户账号", "account":
			if columns.account < 0 {
				columns.account = i
			}
		case "用户名", "姓名", "name", "username":
			if columns.name < 0 {
				columns.name = i
			}
		case "角色", "角色名称", "角色编码", "角色编号", "role", "rolename", "rolecode", "roleid":
			if columns.role < 0 {
				columns.role = i
			}
		case "部门", "部门名称", "部门编码", "部门编号", "department", "departmentname", "departmentcode", "departmentid":
			if columns.department < 0 {
				columns.department = i
			}
		}
	}
	if columns.account < 0 || columns.name < 0 {
		return columns, fmt.Errorf("Excel 第一行必须包含“工号”和“用户名”列")
	}
	return columns, nil
}

func normalizeUserImportHeader(value string) string {
	replacer := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "　", "", "（", "", "）", "", "(", "", ")", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
}

func userImportCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func isBlankUserImportRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
