// 文件说明：ThingsBoard 设备目录缓存。

package qms

import (
	"sort"
	"sync"
	"time"
)

type DeviceInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type DeviceCatalog struct {
	client *ThingsBoardClient
	mu     sync.Mutex
	cache  []DeviceInfo
	at     time.Time
}

func NewDeviceCatalog(client *ThingsBoardClient) *DeviceCatalog {
	return &DeviceCatalog{client: client}
}

func (c *DeviceCatalog) Devices(force bool) ([]DeviceInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && !c.at.IsZero() && time.Since(c.at) < 5*time.Minute {
		return append([]DeviceInfo(nil), c.cache...), nil
	}
	customerID := ""
	if profile, err := c.client.UserProfile(); err == nil {
		if customer, ok := profile["customerId"].(map[string]any); ok {
			customerID, _ = customer["id"].(string)
		}
	}
	list := []DeviceInfo{}
	for page := 0; page < 60; page++ {
		var result map[string]any
		var err error
		if customerID != "" {
			result, err = c.client.CustomerDevices(customerID, page, 100)
		} else {
			result, err = c.client.TenantDevices(page, 100)
		}
		if err != nil {
			return nil, err
		}
		data, _ := result["data"].([]any)
		if len(data) == 0 {
			break
		}
		for _, raw := range data {
			item, _ := raw.(map[string]any)
			idObj, _ := item["id"].(map[string]any)
			id, _ := idObj["id"].(string)
			if id == "" {
				continue
			}
			list = append(list, DeviceInfo{
				ID: id, Name: stringValue(item["name"]), Label: stringValue(item["label"]), Type: stringValue(item["type"]),
			})
		}
		hasNext, _ := result["hasNext"].(bool)
		if !hasNext {
			break
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	c.cache = append([]DeviceInfo(nil), list...)
	c.at = time.Now()
	return list, nil
}

func (c *DeviceCatalog) Find(deviceID string) *DeviceInfo {
	if deviceID == "" {
		return nil
	}
	list, err := c.Devices(false)
	if err != nil {
		return nil
	}
	for _, item := range list {
		if item.ID == deviceID {
			copy := item
			return &copy
		}
	}
	return nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
