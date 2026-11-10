package qms

import (
	"encoding/json"
	"fmt"
	"strings"
)

// historyPageStart returns the newest lower boundary guaranteed complete for
// every field. A timestamp cursor, rather than an offset, prevents gaps when
// telemetry fields have different reporting frequencies.
func historyPageStart(series map[string]map[int64]string, size int, start int64) int64 {
	for _, points := range series {
		if len(points) < size {
			continue
		}
		var oldest int64
		for ts := range points {
			if oldest == 0 || ts < oldest {
				oldest = ts
			}
		}
		if oldest > start {
			start = oldest
		}
	}
	return start
}

// A batch size limits individual source requests, never the query's total.
// Preserve raw JSON values for the raw-data viewer and exports.
func (s *HistoryService) readHistoryRaw(deviceID string, keys []string, from, to int64, limit int, order string) (json.RawMessage, error) {
	path := "/api/plugins/telemetry/DEVICE/" + deviceID + "/values/timeseries"
	if limit > 0 {
		return s.client.GetRaw(path, buildTelemetryQuery(keys, from, to, "NONE", limit, order))
	}
	result := map[string][]rawPoint{}
	for _, key := range keys {
		result[key] = []rawPoint{}
	}
	if len(keys) == 0 {
		return json.Marshal(result)
	}
	desc := !strings.EqualFold(order, "ASC")
	for from <= to {
		payload, err := s.client.GetRaw(path, buildTelemetryQuery(keys, from, to, "NONE", historyBatchSize, order))
		if err != nil {
			return nil, err
		}
		var page map[string][]rawPoint
		if err := json.Unmarshal(payload, &page); err != nil {
			return nil, fmt.Errorf("解析历史分批数据失败: %w", err)
		}
		boundary := from
		if !desc {
			boundary = to
		}
		more := false
		for _, points := range page {
			if len(points) < historyBatchSize {
				continue
			}
			tail := points[0].TS
			for _, point := range points {
				if desc && point.TS < tail || !desc && point.TS > tail {
					tail = point.TS
				}
			}
			if desc && tail > boundary || !desc && tail < boundary {
				boundary = tail
			}
			more = true
		}
		for key, points := range page {
			for _, point := range points {
				if point.TS < from || point.TS > to {
					continue
				}
				if desc && point.TS < boundary || !desc && point.TS > boundary {
					continue
				}
				result[key] = append(result[key], point)
			}
		}
		if !more {
			break
		}
		if desc {
			to = boundary - 1
		} else {
			from = boundary + 1
		}
	}
	return json.Marshal(result)
}
