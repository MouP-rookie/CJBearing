package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) listProducts(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	page, size := pageParams(c)
	var total int64
	if err := scope.Filter(s.db.Model(&model.Product{}), "department_id").Count(&total).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产品失败", err.Error())
		return
	}
	var products []model.Product
	query := s.db.Preload("Department").Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("level, code, id") })
	if err := scope.Filter(query, "department_id").Order("department_id, code, id").Offset((page - 1) * size).Limit(size).Find(&products).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产品失败", err.Error())
		return
	}
	for index := range products {
		lines := products[index].Lines[:0]
		for _, line := range products[index].Lines {
			if line.DepartmentID == products[index].DepartmentID {
				lines = append(lines, line)
			}
		}
		products[index].Lines = lines
		sortLinesByCode(products[index].Lines)
	}
	response.OK(c, gin.H{"items": products, "total": total, "page": page, "size": size})
}

type productRequest struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Enabled *bool    `json:"enabled"`
	LineIDs []uint64 `json:"lineIds"`
}

func (s *Server) createProduct(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	departmentID, err := scope.WritableDepartmentID(s.db)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	req.Code, req.Name = strings.TrimSpace(req.Code), strings.TrimSpace(req.Name)
	if req.Code == "" || req.Name == "" {
		response.Fail(c, 400, "validation_error", "产品编码和名称不能为空", nil)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	product := model.Product{DepartmentID: departmentID, Code: req.Code, Name: req.Name, Enabled: enabled}
	if err := s.db.Create(&product).Error; err != nil {
		response.Fail(c, 409, "create_failed", "产品编码可能已存在", err.Error())
		return
	}
	if err := s.replaceProductLines(&product, req.LineIDs); err != nil {
		response.Fail(c, 500, "save_failed", "保存产品产线关联失败", err.Error())
		return
	}
	s.audit(c, "product", product.ID, "create", nil, product)
	response.Created(c, product)
}

func (s *Server) updateProduct(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var product model.Product
	if err := scope.Filter(s.db, "department_id").First(&product, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "产品不存在", nil)
		return
	}
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	before := product
	if strings.TrimSpace(req.Name) != "" {
		product.Name = strings.TrimSpace(req.Name)
	}
	if req.Enabled != nil {
		product.Enabled = *req.Enabled
	}
	if err := s.db.Save(&product).Error; err != nil {
		response.Fail(c, 500, "save_failed", "保存产品失败", err.Error())
		return
	}
	if req.LineIDs != nil {
		if err := s.replaceProductLines(&product, req.LineIDs); err != nil {
			response.Fail(c, 500, "save_failed", "保存产品产线关联失败", err.Error())
			return
		}
	}
	s.audit(c, "product", product.ID, "update", before, product)
	response.OK(c, product)
}

func (s *Server) deleteProduct(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	if _, err := scope.WritableDepartmentID(s.db); err != nil {
		response.Fail(c, http.StatusBadRequest, "department_required", err.Error(), nil)
		return
	}
	id, ok := parseUint64(c, "id")
	if !ok {
		return
	}
	var product model.Product
	if err := scope.Filter(s.db, "department_id").First(&product, id).Error; err != nil {
		response.Fail(c, 404, "not_found", "产品不存在", nil)
		return
	}
	_ = s.db.Model(&product).Association("Lines").Clear()
	if err := s.db.Delete(&product).Error; err != nil {
		response.Fail(c, 500, "delete_failed", "删除产品失败", err.Error())
		return
	}
	s.audit(c, "product", product.ID, "delete", product, nil)
	response.NoContent(c)
}

// replaceProductLines 保存产品与产线的关联；选择父产线时自动同步其全部子产线。
func (s *Server) replaceProductLines(product *model.Product, lineIDs []uint64) error {
	expanded, err := s.expandLineIDs(lineIDs)
	if err != nil {
		return err
	}
	lines := make([]model.Line, 0, len(expanded))
	if len(expanded) > 0 {
		if err := s.db.Where("id IN ? AND department_id = ?", expanded, product.DepartmentID).Find(&lines).Error; err != nil {
			return err
		}
		if len(lines) != len(expanded) {
			return gorm.ErrRecordNotFound
		}
	}
	return s.db.Model(product).Association("Lines").Replace(lines)
}

// expandLineIDs 返回给定产线及其全部未删除的子产线（产线最多两级）。
func (s *Server) expandLineIDs(lineIDs []uint64) ([]uint64, error) {
	if len(lineIDs) == 0 {
		return lineIDs, nil
	}
	var childIDs []uint64
	if err := s.db.Model(&model.Line{}).Where("parent_id IN ? AND deleted_at IS NULL", lineIDs).Pluck("id", &childIDs).Error; err != nil {
		return nil, err
	}
	seen := make(map[uint64]bool, len(lineIDs)+len(childIDs))
	out := make([]uint64, 0, len(lineIDs)+len(childIDs))
	for _, id := range append(append([]uint64{}, lineIDs...), childIDs...) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *Server) listProductLineOptions(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	var lines []model.Line
	if err := scope.Filter(s.db.Where("enabled = ?", true), "department_id").Order("department_id, level, code, id").Find(&lines).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产线失败", err.Error())
		return
	}
	sortLinesByDepartmentAndCode(lines)
	response.OK(c, gin.H{"items": lines})
}

func (s *Server) executionProducts(c *gin.Context) {
	scope, ok := s.scopeOrFail(c)
	if !ok {
		return
	}
	lineID, _ := strconv.ParseUint(c.Query("lineId"), 10, 64)
	if lineID == 0 || !s.canLineInScope(scope, rbac.PermExecutionView, lineID) {
		response.Fail(c, 403, "forbidden", "没有该产线权限", nil)
		return
	}
	var products []model.Product
	if err := s.db.Joins("JOIN product_lines pl ON pl.product_id = products.id").Where("pl.line_id = ? AND products.enabled = ?", lineID, true).Order("products.code, products.id").Find(&products).Error; err != nil {
		response.Fail(c, 500, "db_error", "查询产品失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": products})
}
