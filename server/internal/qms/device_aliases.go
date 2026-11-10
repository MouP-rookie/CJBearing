// 文件说明：设备改号映射的加载，以及历史查询/导出时的旧设备数据合并。

package qms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// legacyAlias 表示当前设备在 SwitchTs 之前的遥测记录在旧设备条目 LegacyDeviceID 下。
// SwitchTs 取当前设备第一条数据的时间（旧条目在该时间之前的数据归入当前设备）。
type legacyAlias struct {
	DeviceID       string
	LegacyDeviceID string
	SwitchTs       int64
}

type deviceAliasEntry struct {
	DeviceID       string `json:"deviceId"`
	LegacyDeviceID string `json:"legacyDeviceId"`
	SwitchAt       string `json:"switchAt"`
}

type deviceAliasDocument struct {
	Aliases []deviceAliasEntry `json:"aliases"`
}

// LoadDeviceAliases 读取设备改号映射文件；文件不存在时返回空映射，不影响服务启动。
func LoadDeviceAliases(path string) (map[string][]legacyAlias, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取设备改号映射失败: %w", err)
	}
	var document deviceAliasDocument
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, fmt.Errorf("解析设备改号映射失败: %w", err)
	}
	aliases := map[string][]legacyAlias{}
	for _, item := range document.Aliases {
		deviceID := strings.TrimSpace(item.DeviceID)
		legacyID := strings.TrimSpace(item.LegacyDeviceID)
		if deviceID == "" || legacyID == "" || deviceID == legacyID {
			continue
		}
		switchAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.SwitchAt))
		if err != nil {
			return nil, fmt.Errorf("设备改号映射 switchAt 无法解析（%s）: %w", deviceID, err)
		}
		aliases[deviceID] = append(aliases[deviceID], legacyAlias{
			DeviceID: deviceID, LegacyDeviceID: legacyID, SwitchTs: switchAt.UnixMilli(),
		})
	}
	return aliases, nil
}

// legacySegments 返回本次查询区间内需要从旧设备条目补数据的映射。
func (s *HistoryService) legacySegments(deviceID string, from, to int64) []legacyAlias {
	if s == nil || len(s.aliases) == 0 {
		return nil
	}
	segments := s.aliases[deviceID]
	if len(segments) == 0 {
		return nil
	}
	wanted := make([]legacyAlias, 0, len(segments))
	for _, segment := range segments {
		if segment.SwitchTs <= from || from > to {
			continue
		}
		wanted = append(wanted, segment)
	}
	return wanted
}

// legacyRange 计算旧设备条目的查询终点：改号切换时间之前。
func legacyRange(from, to, switchTs int64) (int64, bool) {
	end := to
	if switchTs-1 < end {
		end = switchTs - 1
	}
	if from > end {
		return 0, false
	}
	return end, true
}

// unionKeys 合并设备字段列表，保持首次出现顺序。
func unionKeys(lists ...[]string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, list := range lists {
		for _, key := range list {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, key)
		}
	}
	return result
}

// mergeSeries 把旧设备条目中早于改号切换时间的点补进当前设备的数据里；
// 同一字段同一时间戳两边都有值时保留当前设备的值。
func mergeSeries(base, legacy map[string]map[int64]string, switchTs int64) map[string]map[int64]string {
	if len(legacy) == 0 {
		return base
	}
	if base == nil {
		base = map[string]map[int64]string{}
	}
	for key, points := range legacy {
		target, ok := base[key]
		if !ok {
			target = map[int64]string{}
			base[key] = target
		}
		for ts, value := range points {
			if switchTs > 0 && ts >= switchTs {
				continue
			}
			if _, exists := target[ts]; !exists {
				target[ts] = value
			}
		}
	}
	return base
}

// rawPoint 是原始报文中的一个遥测点；Value 保持原始 JSON，避免数值/字符串类型被改写。
type rawPoint struct {
	TS    int64           `json:"ts"`
	Value json.RawMessage `json:"value"`
}

// rawSource 表示一份待合并的原始报文；Until 大于 0 时只保留早于该时间戳的点。
type rawSource struct {
	Payload json.RawMessage
	Until   int64
}

// mergeRawPayloads 合并当前设备与旧设备条目的原始报文：按字段去重，同一字段同一时间戳保留先出现的来源，
// 并按前端请求的顺序（默认倒序）排序；limit 与 ThingsBoard 一致按每个字段的条数截断。
func mergeRawPayloads(sources []rawSource, keys []string, limit int, desc bool) (json.RawMessage, error) {
	merged := map[string][]rawPoint{}
	seen := map[string]map[int64]bool{}
	for _, source := range sources {
		if len(bytes.TrimSpace(source.Payload)) == 0 {
			continue
		}
		var data map[string][]rawPoint
		if err := json.Unmarshal(source.Payload, &data); err != nil {
			return nil, fmt.Errorf("解析遥测原始报文失败: %w", err)
		}
		for key, points := range data {
			if len(points) == 0 {
				continue
			}
			marks := seen[key]
			if marks == nil {
				marks = map[int64]bool{}
				seen[key] = marks
			}
			list := merged[key]
			for _, point := range points {
				if source.Until > 0 && point.TS >= source.Until {
					continue
				}
				if marks[point.TS] {
					continue
				}
				marks[point.TS] = true
				list = append(list, point)
			}
			merged[key] = list
		}
	}
	for key, points := range merged {
		sort.Slice(points, func(i, j int) bool {
			if desc {
				return points[i].TS > points[j].TS
			}
			return points[i].TS < points[j].TS
		})
		if limit > 0 && len(points) > limit {
			points = points[:limit]
		}
		merged[key] = points
	}
	// 没有数据的字段也返回空数组，保持与 ThingsBoard 原始报文一致的字段集合。
	for _, key := range keys {
		if _, ok := merged[key]; !ok {
			merged[key] = []rawPoint{}
		}
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}
