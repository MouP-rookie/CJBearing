package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const lineImportMaxBytes = 10 << 20

type lineImportColumns struct {
	code   int
	name   int
	parent int
	status int
}

type lineImportRow struct {
	rowNo      int
	code       string
	name       string
	parentCode string
	enabled    bool
}

type lineImportRowError struct {
	Row     int    `json:"row"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (s *Server) lineImportTemplate(c *gin.Context) {
	f := excelize.NewFile()
	sheet := "产线导入"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		response.Fail(c, http.StatusInternalServerError, "template_failed", "生成导入模板失败", err.Error())
		return
	}
	_ = f.SetSheetRow(sheet, "A1", &[]any{"产线编码", "产线名称", "上级产线编码", "状态"})
	_ = f.SetSheetRow(sheet, "A2", &[]any{"L001", "一号产线", "", "启用"})
	_ = f.SetSheetRow(sheet, "A3", &[]any{"L001-01", "一号产线首检工位", "L001", "启用"})
	_ = f.SetColWidth(sheet, "A", "D", 22)
	writeWorkbook(c, f, "产线批量导入模板.xlsx")
}

func (s *Server) importLines(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if scope.AllDepartments {
		response.Fail(c, http.StatusBadRequest, "department_required", "请先选择部门", nil)
		return
	}
	departmentID, err := scope.WritableDepartmentID(s.db)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, lineImportMaxBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "请选择 Excel 文件", err.Error())
		return
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".xlsx") {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "只支持 .xlsx 格式的 Excel 文件", nil)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "无法读取 Excel 文件", err.Error())
		return
	}
	defer file.Close()
	workbook, err := excelize.OpenReader(file)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "Excel 文件格式不正确", err.Error())
		return
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "Excel 中没有可读取的工作表", nil)
		return
	}
	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "读取 Excel 内容失败", err.Error())
		return
	}
	if len(rows) == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid_file", "Excel 中没有数据", nil)
		return
	}
	columns, err := parseLineImportHeader(rows[0])
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid_header", err.Error(), nil)
		return
	}

	existingByCode := map[string]model.Line{}
	var existingLines []model.Line
	if err := s.db.Where("department_id = ?", departmentID).Find(&existingLines).Error; err != nil {
		response.Fail(c, http.StatusInternalServerError, "db_error", "查询现有产线失败", err.Error())
		return
	}
	for _, line := range existingLines {
		existingByCode[line.Code] = line
	}

	parsed := make([]lineImportRow, 0, len(rows)-1)
	rowErrors := make([]lineImportRowError, 0)
	invalidCodes := map[string]bool{}
	codeRows := map[string]int{}
	for index := 1; index < len(rows); index++ {
		row := rows[index]
		if isBlankLineImportRow(row) {
			continue
		}
		rowNo := index + 1
		code := lineImportCell(row, columns.code)
		name := lineImportCell(row, columns.name)
		parentCode := lineImportCell(row, columns.parent)
		enabled, enabledErr := parseLineImportEnabled(lineImportCell(row, columns.status))
		if code == "" {
			rowErrors = append(rowErrors, lineImportRowError{Row: rowNo, Code: code, Name: name, Message: "产线编码不能为空"})
			continue
		}
		if name == "" {
			rowErrors = append(rowErrors, lineImportRowError{Row: rowNo, Code: code, Name: name, Message: "产线名称不能为空"})
			invalidCodes[code] = true
			continue
		}
		if enabledErr != nil {
			rowErrors = append(rowErrors, lineImportRowError{Row: rowNo, Code: code, Name: name, Message: enabledErr.Error()})
			invalidCodes[code] = true
			continue
		}
		if firstRow, exists := codeRows[code]; exists {
			rowErrors = append(rowErrors, lineImportRowError{Row: rowNo, Code: code, Name: name, Message: fmt.Sprintf("产线编码与第%d行重复", firstRow)})
			invalidCodes[code] = true
			continue
		}
		codeRows[code] = rowNo
		if _, exists := existingByCode[code]; exists {
			rowErrors = append(rowErrors, lineImportRowError{Row: rowNo, Code: code, Name: name, Message: "当前部门已存在相同产线编码"})
			invalidCodes[code] = true
			continue
		}
		parsed = append(parsed, lineImportRow{rowNo: rowNo, code: code, name: name, parentCode: parentCode, enabled: enabled})
	}

	levelOneCodes := map[string]bool{}
	for _, row := range parsed {
		if !invalidCodes[row.code] && row.parentCode == "" {
			levelOneCodes[row.code] = true
		}
	}
	for _, row := range parsed {
		if invalidCodes[row.code] {
			continue
		}
		if row.parentCode == "" {
			continue
		}
		if row.parentCode == row.code {
			invalidCodes[row.code] = true
			rowErrors = append(rowErrors, lineImportRowError{Row: row.rowNo, Code: row.code, Name: row.name, Message: "上级产线不能是自身"})
			continue
		}
		if parent, exists := existingByCode[row.parentCode]; exists {
			if parent.Level != 1 {
				invalidCodes[row.code] = true
				rowErrors = append(rowErrors, lineImportRowError{Row: row.rowNo, Code: row.code, Name: row.name, Message: "产线最多两级，上级产线必须是根产线"})
			}
			continue
		}
		if levelOneCodes[row.parentCode] {
			continue
		}
		invalidCodes[row.code] = true
		rowErrors = append(rowErrors, lineImportRowError{Row: row.rowNo, Code: row.code, Name: row.name, Message: "上级产线编码不存在或父产线行有错误"})
	}

	parentIDs := map[string]uint64{}
	for code, line := range existingByCode {
		if line.Level == 1 {
			parentIDs[code] = line.ID
		}
	}
	created := 0
	createdLines := make([]model.Line, 0, len(parsed))
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, row := range parsed {
			if invalidCodes[row.code] || row.parentCode != "" {
				continue
			}
			line := model.Line{DepartmentID: departmentID, Code: row.code, Name: row.name, Level: 1, Enabled: row.enabled}
			if err := tx.Create(&line).Error; err != nil {
				return err
			}
			parentIDs[row.code] = line.ID
			created++
			createdLines = append(createdLines, line)
		}
		for _, row := range parsed {
			if invalidCodes[row.code] || row.parentCode == "" {
				continue
			}
			parentID, exists := parentIDs[row.parentCode]
			if !exists {
				return fmt.Errorf("第%d行上级产线不存在", row.rowNo)
			}
			line := model.Line{DepartmentID: departmentID, ParentID: &parentID, Code: row.code, Name: row.name, Level: 2, Enabled: row.enabled}
			if err := tx.Create(&line).Error; err != nil {
				return err
			}
			created++
			createdLines = append(createdLines, line)
		}
		return nil
	})
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "import_failed", "导入产线失败", err.Error())
		return
	}
	for _, line := range createdLines {
		s.audit(c, "line", line.ID, "import", nil, line)
	}
	response.OK(c, gin.H{"created": created, "failed": len(rowErrors), "errors": rowErrors})
}

func parseLineImportHeader(row []string) (lineImportColumns, error) {
	columns := lineImportColumns{code: -1, name: -1, parent: -1, status: -1}
	for index, value := range row {
		switch normalizeUserImportHeader(value) {
		case "产线编码", "编码", "code", "linecode":
			if columns.code < 0 {
				columns.code = index
			}
		case "产线名称", "名称", "name", "linename":
			if columns.name < 0 {
				columns.name = index
			}
		case "上级产线编码", "父产线编码", "上级编码", "parentcode", "parent":
			if columns.parent < 0 {
				columns.parent = index
			}
		case "状态", "启用状态", "是否启用", "enabled", "status":
			if columns.status < 0 {
				columns.status = index
			}
		}
	}
	if columns.code < 0 || columns.name < 0 {
		return columns, errors.New("Excel 第一行必须包含“产线编码”和“产线名称”列")
	}
	return columns, nil
}

func parseLineImportEnabled(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "启用", "是", "true", "1", "yes":
		return true, nil
	case "停用", "否", "false", "0", "no":
		return false, nil
	default:
		return false, errors.New("状态仅支持“启用/停用”或留空")
	}
}

func lineImportCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func isBlankLineImportRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
