// 文件说明：Go 版 ThingsBoard 客户端，负责登录缓存、遥测和设备目录请求。

package qms

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ThingsBoardClient struct {
	baseURL  string
	username string
	password string
	http     *http.Client

	mu             sync.Mutex
	token          string
	tokenExpiresAt time.Time
}

func NewThingsBoardClient(baseURL, username, password string) *ThingsBoardClient {
	return &ThingsBoardClient{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		username: username,
		password: password,
		http:     &http.Client{Timeout: 45 * time.Second},
	}
}

func (c *ThingsBoardClient) Token() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.tokenExpiresAt) > time.Minute {
		return c.token, nil
	}
	if c.username == "" || c.password == "" {
		return "", fmt.Errorf("未配置 IOT_USERNAME / IOT_PASSWORD")
	}
	body, _ := json.Marshal(map[string]string{"username": c.username, "password": c.password})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ThingsBoard 登录失败: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ThingsBoard 登录失败: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		return "", fmt.Errorf("ThingsBoard 登录响应解析失败: %w", err)
	}
	if result.Token == "" {
		return "", fmt.Errorf("ThingsBoard 登录响应中没有 token")
	}
	c.token = result.Token
	c.tokenExpiresAt = jwtExpiry(result.Token)
	if c.tokenExpiresAt.IsZero() {
		c.tokenExpiresAt = time.Now().Add(30 * time.Minute)
	}
	return c.token, nil
}

func (c *ThingsBoardClient) Invalidate() {
	c.mu.Lock()
	c.token = ""
	c.tokenExpiresAt = time.Time{}
	c.mu.Unlock()
}

func (c *ThingsBoardClient) GetJSON(path string, query url.Values) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.Token()
		if err != nil {
			return nil, err
		}
		target := c.baseURL + path
		if len(query) > 0 {
			target += "?" + query.Encode()
		}
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ThingsBoard 请求失败: %w", err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			c.Invalidate()
			lastErr = fmt.Errorf("ThingsBoard 鉴权失败: HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("ThingsBoard 查询失败: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(payload)))
		}
		if len(bytes.TrimSpace(payload)) == 0 {
			return map[string]any{}, nil
		}
		var result map[string]any
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil, fmt.Errorf("ThingsBoard 响应解析失败: %w", err)
		}
		return result, nil
	}
	return nil, lastErr
}

func (c *ThingsBoardClient) GetRaw(path string, query url.Values) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.Token()
		if err != nil {
			return nil, err
		}
		target := c.baseURL + path
		if len(query) > 0 {
			target += "?" + query.Encode()
		}
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("X-Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			c.Invalidate()
			lastErr = fmt.Errorf("ThingsBoard 鉴权失败: HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("ThingsBoard 查询失败: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(payload)))
		}
		if len(bytes.TrimSpace(payload)) == 0 {
			return json.RawMessage("{}"), nil
		}
		return json.RawMessage(payload), nil
	}
	return nil, lastErr
}

func (c *ThingsBoardClient) Timeseries(deviceID string, keys []string, start, end int64, agg string, limit int, order string) (map[string]any, error) {
	query := url.Values{}
	if len(keys) > 0 {
		query.Set("keys", strings.Join(keys, ","))
	}
	query.Set("startTs", fmt.Sprint(start))
	query.Set("endTs", fmt.Sprint(end))
	if agg != "" {
		query.Set("agg", agg)
	}
	if limit > 0 {
		query.Set("limit", fmt.Sprint(limit))
	}
	if order != "" {
		query.Set("orderBy", order)
	}
	return c.GetJSON("/api/plugins/telemetry/DEVICE/"+url.PathEscape(deviceID)+"/values/timeseries", query)
}

func (c *ThingsBoardClient) TimeseriesKeys(deviceID string) ([]string, error) {
	payload, err := c.GetRaw("/api/plugins/telemetry/DEVICE/"+url.PathEscape(deviceID)+"/keys/timeseries", nil)
	if err != nil {
		return nil, err
	}
	var keys []string
	if err := json.Unmarshal(payload, &keys); err != nil {
		return []string{}, nil
	}
	return keys, nil
}

func (c *ThingsBoardClient) UserProfile() (map[string]any, error) {
	return c.GetJSON("/api/auth/user", nil)
}

func (c *ThingsBoardClient) CustomerDevices(customerID string, page, size int) (map[string]any, error) {
	return c.GetJSON("/api/customer/"+url.PathEscape(customerID)+"/devices", paginationQuery(page, size))
}

func (c *ThingsBoardClient) TenantDevices(page, size int) (map[string]any, error) {
	return c.GetJSON("/api/tenant/devices", paginationQuery(page, size))
}

func paginationQuery(page, size int) url.Values {
	query := url.Values{}
	query.Set("page", fmt.Sprint(page))
	query.Set("pageSize", fmt.Sprint(size))
	query.Set("sortProperty", "createdTime")
	query.Set("sortOrder", "ASC")
	return query
}

func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var payload struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(payload.Exp, 0)
}
