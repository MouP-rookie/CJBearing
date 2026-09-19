package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"bearing-inspection/internal/httpapi/response"
	"bearing-inspection/internal/model"
	"bearing-inspection/internal/rbac"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	brandLogoKey   = "brand_logo"
	voidReasonsKey = "submission_void_reasons"
	maxLogoBytes   = 2 << 20
	maxVoidReasons = 50
	maxReasonRunes = 50
)

var defaultVoidReasons = []string{"录入错误", "重复提交", "数据无效", "操作失误"}

type brandSettingsRequest struct {
	LogoDataURL string `json:"logoDataUrl"`
}

type voidReasonsRequest struct {
	Items []string `json:"items"`
}

func (s *Server) getBrandSettings(c *gin.Context) {
	var setting model.AppSetting
	err := s.db.Where("key = ?", brandLogoKey).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.OK(c, gin.H{"logoDataUrl": ""})
		return
	}
	if err != nil {
		response.Fail(c, 500, "db_error", "读取系统设置失败", err.Error())
		return
	}
	response.OK(c, gin.H{"logoDataUrl": setting.Value, "updatedAt": setting.UpdatedAt})
}

func (s *Server) updateBrandSettings(c *gin.Context) {
	var req brandSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	req.LogoDataURL = strings.TrimSpace(req.LogoDataURL)
	if req.LogoDataURL != "" {
		if err := validateLogoDataURL(req.LogoDataURL); err != nil {
			response.Fail(c, 400, "validation_error", err.Error(), nil)
			return
		}
	}
	if err := s.saveAppSetting(brandLogoKey, req.LogoDataURL); err != nil {
		response.Fail(c, 500, "save_failed", "保存系统设置失败", err.Error())
		return
	}
	s.audit(c, "app_setting", 0, "update_brand_logo", nil, gin.H{"hasLogo": req.LogoDataURL != ""})
	response.OK(c, gin.H{"logoDataUrl": req.LogoDataURL, "updatedAt": time.Now()})
}

func (s *Server) getVoidReasons(c *gin.Context) {
	items, err := s.readVoidReasons()
	if err != nil {
		response.Fail(c, 500, "db_error", "读取废除原因失败", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (s *Server) updateVoidReasons(c *gin.Context) {
	uid := c.GetUint64("user_id")
	systemAllowed, systemErr := s.authorizer.Can(uid, rbac.PermSystemManage, nil)
	logAllowed, logErr := s.authorizer.Can(uid, rbac.PermLogView, nil)
	if (systemErr != nil || !systemAllowed) && (logErr != nil || !logAllowed) {
		response.Fail(c, 403, "forbidden", "没有权限维护废除原因", nil)
		return
	}
	var req voidReasonsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "invalid_request", "参数错误", err.Error())
		return
	}
	items, err := normalizeVoidReasons(req.Items)
	if err != nil {
		response.Fail(c, 400, "validation_error", err.Error(), nil)
		return
	}
	raw, err := json.Marshal(items)
	if err != nil {
		response.Fail(c, 500, "save_failed", "保存废除原因失败", err.Error())
		return
	}
	if err := s.saveAppSetting(voidReasonsKey, string(raw)); err != nil {
		response.Fail(c, 500, "save_failed", "保存废除原因失败", err.Error())
		return
	}
	s.audit(c, "app_setting", 0, "update_void_reasons", nil, gin.H{"items": items})
	response.OK(c, gin.H{"items": items, "updatedAt": time.Now()})
}

func (s *Server) readVoidReasons() ([]string, error) {
	var setting model.AppSetting
	err := s.db.Where("key = ?", voidReasonsKey).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && strings.TrimSpace(setting.Value) == "") {
		return append([]string(nil), defaultVoidReasons...), nil
	}
	if err != nil {
		return nil, err
	}
	var items []string
	if err := json.Unmarshal([]byte(setting.Value), &items); err != nil {
		return nil, fmt.Errorf("废除原因配置格式无效: %w", err)
	}
	return normalizeVoidReasons(items)
}

func normalizeVoidReasons(raw []string) ([]string, error) {
	if len(raw) > maxVoidReasons {
		return nil, fmt.Errorf("废除原因最多配置%d项", maxVoidReasons)
	}
	items := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for index, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("第%d项废除原因不能为空", index+1)
		}
		if value == "其他" {
			return nil, errors.New("“其他”为系统保留选项，无需配置")
		}
		if utf8.RuneCountInString(value) > maxReasonRunes {
			return nil, fmt.Errorf("第%d项废除原因不能超过%d字", index+1, maxReasonRunes)
		}
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("废除原因“%s”重复", value)
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items, nil
}

func (s *Server) saveAppSetting(key, value string) error {
	var setting model.AppSetting
	err := s.db.Where("key = ?", key).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.Create(&model.AppSetting{Key: key, Value: value, UpdatedAt: time.Now()}).Error
	}
	if err != nil {
		return err
	}
	setting.Value = value
	setting.UpdatedAt = time.Now()
	return s.db.Save(&setting).Error
}

func validateLogoDataURL(value string) error {
	allowed := []string{
		"data:image/png;base64,",
		"data:image/jpeg;base64,",
		"data:image/webp;base64,",
	}
	allowedPrefix := ""
	for _, prefix := range allowed {
		if strings.HasPrefix(value, prefix) {
			allowedPrefix = prefix
			break
		}
	}
	if allowedPrefix == "" {
		return errors.New("Logo 仅支持 PNG、JPG 或 WebP 图片")
	}
	payload := value[len(allowedPrefix):]
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return errors.New("Logo 图片数据无效")
	}
	if len(decoded) > maxLogoBytes {
		return errors.New("Logo 图片不能超过 2MB")
	}
	return nil
}
