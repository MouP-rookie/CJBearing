// 文件说明：开关机推导、压差和温湿度实时数据。

package qms

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bearing-inspection/internal/config"
)

const (
	qmsKeyStatus      = "status"
	qmsKeyCode        = "code"
	qmsKeyPressure1   = "负压1号机"
	qmsKeyPressure2   = "负压2号机"
	qmsKeyPressure3   = "负压3号机"
	qmsKeyPressure4   = "负压4号机"
	qmsKeyTemperature = "温度"
	qmsKeyHumidity    = "湿度"
	qmsKeyCollection  = "CollectionTime"
)

type PowerRecord struct {
	Seq             int     `json:"seq"`
	StartTs         int64   `json:"startTs"`
	StartTime       string  `json:"startTime"`
	EndTs           *int64  `json:"endTs"`
	EndTime         *string `json:"endTime"`
	Running         bool    `json:"running"`
	DurationSeconds int64   `json:"durationSeconds"`
	DurationText    string  `json:"durationText"`
	Reason          string  `json:"reason"`
}

type RawItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Time  string `json:"time"`
}

type PressureRow struct {
	Seq         int       `json:"seq"`
	DeviceName  string    `json:"deviceName"`
	Diff1       *string   `json:"diff1"`
	Diff2       *string   `json:"diff2"`
	CollectTime string    `json:"collectTime"`
	StatusText  string    `json:"statusText"`
	Raw         []RawItem `json:"raw"`
}

type THRow struct {
	Seq         int       `json:"seq"`
	DeviceName  string    `json:"deviceName"`
	Temperature *string   `json:"temperature"`
	Humidity    *string   `json:"humidity"`
	CollectTime string    `json:"collectTime"`
	Raw         []RawItem `json:"raw"`
}

type TelemetryService struct {
	client *ThingsBoardClient
	cfg    *config.Config
	mu     sync.Mutex
	cache  []PowerRecord
	at     time.Time
}

func NewTelemetryService(client *ThingsBoardClient, cfg *config.Config) *TelemetryService {
	return &TelemetryService{client: client, cfg: cfg}
}

func (s *TelemetryService) PowerRecords(force bool, from, to *int64) ([]PowerRecord, error) {
	all, err := s.allPowerRecords(force)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	rows := make([]PowerRecord, 0, len(all))
	for _, record := range all {
		recordEnd := now
		if record.EndTs != nil {
			recordEnd = *record.EndTs
		}
		if from != nil && recordEnd < *from {
			continue
		}
		if to != nil && record.StartTs > *to {
			continue
		}
		record.Seq = len(rows) + 1
		rows = append(rows, record)
	}
	return rows, nil
}

func (s *TelemetryService) allPowerRecords(force bool) ([]PowerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !force && !s.at.IsZero() && time.Since(s.at) < 120*time.Second {
		return append([]PowerRecord(nil), s.cache...), nil
	}
	start := parsePowerStart(s.cfg.IOTPowerStart)
	now := time.Now().UnixMilli()
	data, err := s.client.Timeseries(s.cfg.IOTPressureID, []string{qmsKeyStatus}, start, now, "NONE", s.cfg.IOTQueryLimit, "ASC")
	if err != nil {
		return nil, err
	}
	points := seriesPoints(data, qmsKeyStatus)
	sort.Slice(points, func(i, j int) bool { return points[i].TS < points[j].TS })
	gap := int64(max(1, s.cfg.IOTGapMinutes)) * int64(time.Minute/time.Millisecond)

	chronological := []PowerRecord{}
	var openedAt *int64
	var previous *telemetryPoint
	for i := range points {
		point := &points[i]
		on := parsePowerState(point.Value)
		reportedOff := on != nil && !*on
		interrupted := previous != nil && openedAt != nil && point.TS-previous.TS > gap
		if openedAt != nil && reportedOff {
			if point.TS-*openedAt >= 1000 {
				chronological = append(chronological, closedPower(*openedAt, point.TS, "设备上报关机"))
			}
			openedAt = nil
		} else if openedAt != nil && interrupted {
			if previous.TS-*openedAt >= 1000 {
				chronological = append(chronological, closedPower(*openedAt, previous.TS, "数据中断"))
			}
			openedAt = nil
		}
		if on != nil && *on && openedAt == nil {
			ts := point.TS
			openedAt = &ts
		}
		previous = point
	}
	if openedAt != nil {
		seconds := max(0, (now-*openedAt)/1000)
		chronological = append(chronological, PowerRecord{
			StartTs: *openedAt, StartTime: formatTime(*openedAt), Running: true,
			DurationSeconds: seconds, DurationText: durationText(seconds), Reason: "设备运行中",
		})
	}
	sort.SliceStable(chronological, func(i, j int) bool { return chronological[i].StartTs > chronological[j].StartTs })
	for i := range chronological {
		chronological[i].Seq = i + 1
	}
	s.cache = append([]PowerRecord(nil), chronological...)
	s.at = time.Now()
	return chronological, nil
}

func (s *TelemetryService) PressureRows(limit int, from, to *int64) ([]PressureRow, error) {
	start, end, err := qmsWindow(from, to)
	if err != nil {
		return nil, err
	}
	keys := []string{qmsKeyCode, qmsKeyStatus, qmsKeyPressure1, qmsKeyPressure2, qmsKeyPressure3, qmsKeyPressure4}
	data, err := s.client.Timeseries(s.cfg.IOTPressureID, keys, start, end, "NONE", limit, "DESC")
	if err != nil {
		return nil, err
	}
	series := seriesMap(data, keys)
	rows := []PressureRow{}
	for _, ts := range descendingKeys(series[qmsKeyPressure1]) {
		v1 := series[qmsKeyPressure1][ts]
		v2 := nearestFloor(series[qmsKeyPressure2], ts)
		v3 := nearestFloor(series[qmsKeyPressure3], ts)
		status := nearestFloor(series[qmsKeyStatus], ts)
		raw := []RawItem{}
		for _, key := range keys {
			if value, ok := floorEntry(series[key], ts); ok {
				raw = append(raw, RawItem{Key: key, Value: value.Value, Time: formatTime(value.TS)})
			}
		}
		rows = append(rows, PressureRow{
			Seq: len(rows) + 1, DeviceName: s.cfg.IOTPressureLabel,
			Diff1: subtractValues(v1, v2), Diff2: subtractValues(v2, v3),
			CollectTime: formatTime(ts), StatusText: statusText(status), Raw: raw,
		})
		if len(rows) >= limit {
			break
		}
	}
	return rows, nil
}

func (s *TelemetryService) THRows(limit int, from, to *int64) ([]THRow, error) {
	start, end, err := qmsWindow(from, to)
	if err != nil {
		return nil, err
	}
	keys := []string{qmsKeyCode, qmsKeyCollection, qmsKeyTemperature, qmsKeyHumidity}
	data, err := s.client.Timeseries(s.cfg.IOTTHID, keys, start, end, "NONE", limit, "DESC")
	if err != nil {
		return nil, err
	}
	series := seriesMap(data, keys)
	rows := []THRow{}
	for _, ts := range descendingKeys(series[qmsKeyTemperature]) {
		temperature := stringPtr(series[qmsKeyTemperature][ts])
		humidity := stringPtr(series[qmsKeyHumidity][ts])
		collect := series[qmsKeyCollection][ts]
		if collect == "" {
			collect = formatTime(ts)
		}
		raw := []RawItem{}
		for _, key := range keys {
			if value, ok := floorEntry(series[key], ts); ok {
				raw = append(raw, RawItem{Key: key, Value: value.Value, Time: formatTime(value.TS)})
			}
		}
		rows = append(rows, THRow{
			Seq: len(rows) + 1, DeviceName: s.cfg.IOTTHLabel,
			Temperature: temperature, Humidity: humidity, CollectTime: collect, Raw: raw,
		})
		if len(rows) >= limit {
			break
		}
	}
	return rows, nil
}

type telemetryPoint struct {
	TS    int64
	Value string
}

func seriesPoints(data map[string]any, key string) []telemetryPoint {
	values, _ := data[key].([]any)
	points := make([]telemetryPoint, 0, len(values))
	for _, raw := range values {
		item, _ := raw.(map[string]any)
		ts := int64Number(item["ts"])
		points = append(points, telemetryPoint{TS: ts, Value: fmt.Sprint(item["value"])})
	}
	return points
}

func seriesMap(data map[string]any, keys []string) map[string]map[int64]string {
	result := map[string]map[int64]string{}
	for _, key := range keys {
		values := map[int64]string{}
		for _, point := range seriesPoints(data, key) {
			values[point.TS] = point.Value
		}
		result[key] = values
	}
	return result
}

func descendingKeys(values map[int64]string) []int64 {
	keys := make([]int64, 0, len(values))
	for ts := range values {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] > keys[j] })
	return keys
}

func nearestFloor(values map[int64]string, at int64) string {
	value, _ := floorEntry(values, at)
	return value.Value
}

func floorEntry(values map[int64]string, at int64) (telemetryPoint, bool) {
	keys := make([]int64, 0, len(values))
	for ts := range values {
		if ts <= at {
			keys = append(keys, ts)
		}
	}
	if len(keys) == 0 {
		return telemetryPoint{}, false
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] > keys[j] })
	return telemetryPoint{TS: keys[0], Value: values[keys[0]]}, true
}

func qmsWindow(from, to *int64) (int64, int64, error) {
	end := time.Now().UnixMilli()
	if to != nil {
		end = *to
	}
	start := end - int64(30*24*time.Hour/time.Millisecond)
	if from != nil {
		start = *from
	}
	if start > end {
		return 0, 0, fmt.Errorf("开始时间不能晚于结束时间")
	}
	return start, end, nil
}

func parsePowerState(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		result := true
		return &result
	case "false", "5":
		result := false
		return &result
	default:
		return nil
	}
}

func statusText(value string) string {
	state := parsePowerState(value)
	if state == nil {
		return "未知"
	}
	if *state {
		return "运行中"
	}
	return "已停机"
}

func closedPower(start, end int64, reason string) PowerRecord {
	seconds := max(0, (end-start)/1000)
	endTime := formatTime(end)
	return PowerRecord{
		StartTs: start, StartTime: formatTime(start), EndTs: &end, EndTime: &endTime,
		Running: false, DurationSeconds: seconds, DurationText: durationText(seconds), Reason: reason,
	}
}

func durationText(seconds int64) string {
	seconds = max(0, seconds)
	return fmt.Sprintf("%d天%d小时%d分%d秒", seconds/86400, seconds%86400/3600, seconds%3600/60, seconds%60)
}

func subtractValues(a, b string) *string {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return nil
	}
	left, err1 := strconv.ParseFloat(a, 64)
	right, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil {
		return nil
	}
	value := strconv.FormatFloat(math.Round((right-left)*100)/100, 'f', 2, 64)
	return &value
}

func formatTime(ts int64) string {
	location, _ := time.LoadLocation("Asia/Shanghai")
	return time.UnixMilli(ts).In(location).Format("2006-01-02 15:04:05")
}

func parsePowerStart(raw string) int64 {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		fallback, _ := time.Parse(time.RFC3339, "2025-07-09T00:00:00+08:00")
		return fallback.UnixMilli()
	}
	return parsed.UnixMilli()
}

func int64Number(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case jsonNumber:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return parsed
	}
}

type jsonNumber string

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
