// 文件说明：设备历史查询、超限判定与 CSV/JSON 导出。

package qms

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	historyBatchSize = 1000
)

type HistoryService struct {
	client  *ThingsBoardClient
	catalog *DeviceCatalog
	// aliases 保存设备改号映射：当前设备 ID -> 改号前的旧设备条目，用于把改号前的历史遥测并入当前设备。
	aliases map[string][]legacyAlias
}

type HistoryGroup struct {
	Name     string `json:"name"`
	ValueKey string `json:"valueKey"`
	UpperKey string `json:"upperKey,omitempty"`
	LowerKey string `json:"lowerKey,omitempty"`
	HasLimit bool   `json:"hasLimit"`
	Source   string `json:"source"`
}

type historyRow struct {
	TS     int64             `json:"ts"`
	Time   string            `json:"time"`
	Values map[string]string `json:"values"`
	Flags  map[string]string `json:"flags"`
}

type SPCItem struct {
	Name  string `json:"Name"`
	UCL   string `json:"Ucl"`
	LCL   string `json:"Lcl"`
	Value string `json:"Value"`
}

type SPCRow struct {
	TS         int64  `json:"ts"`
	Time       string `json:"time"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	UCL        string `json:"ucl"`
	LCL        string `json:"lcl"`
	Applicable bool   `json:"applicable"`
	Out        bool   `json:"out"`
}

func NewHistoryService(client *ThingsBoardClient, catalog *DeviceCatalog, aliases map[string][]legacyAlias) *HistoryService {
	return &HistoryService{client: client, catalog: catalog, aliases: aliases}
}

func (s *HistoryService) Build(deviceID string, fromParam, toParam *int64, limitParam int, order string) (map[string]any, error) {
	return s.build(deviceID, fromParam, toParam, limitParam, order, false)
}

func (s *HistoryService) BuildPage(deviceID string, from, to *int64) (map[string]any, error) {
	return s.build(deviceID, from, to, historyBatchSize, "DESC", true)
}

func (s *HistoryService) build(deviceID string, fromParam, toParam *int64, limitParam int, order string, paged bool) (map[string]any, error) {
	to := time.Now().UnixMilli()
	if toParam != nil {
		to = *toParam
	}
	from := to - int64(7*24*time.Hour/time.Millisecond)
	if fromParam != nil {
		from = *fromParam
	}
	if from > to {
		return nil, fmt.Errorf("开始时间不能晚于结束时间")
	}
	limit := 0
	if paged {
		limit = historyBatchSize
	}
	desc := !strings.EqualFold(order, "ASC")
	device := s.catalog.Find(deviceID)
	legacy := s.legacySegments(deviceID, from, to)
	keys, err := s.client.TimeseriesKeys(deviceID)
	if err != nil {
		return nil, err
	}
	for _, segment := range legacy {
		legacyKeys, legacyErr := s.client.TimeseriesKeys(segment.LegacyDeviceID)
		if legacyErr != nil {
			slog.Warn("读取改号前设备字段失败，跳过该段历史数据", "deviceId", deviceID, "legacyDeviceId", segment.LegacyDeviceID, "error", legacyErr)
			return nil, fmt.Errorf("读取改号前设备数据失败，无法确认完整范围: %w", legacyErr)
		}
		keys = unionKeys(keys, legacyKeys)
	}
	groups := detectFieldGroups(keys)
	hasSPC := containsString(keys, "spcvalue")
	series, err := s.fetchSeries(deviceID, keys, from, to, limit, desc)
	if err != nil {
		return nil, err
	}
	boundary := from
	if paged {
		boundary = historyPageStart(series, limit, boundary)
	}
	for _, segment := range legacy {
		legacyEnd, ok := legacyRange(from, to, segment.SwitchTs)
		if !ok {
			continue
		}
		part, legacyErr := s.fetchSeries(segment.LegacyDeviceID, keys, from, legacyEnd, limit, desc)
		if legacyErr != nil {
			slog.Warn("读取改号前设备历史失败，跳过该段历史数据", "deviceId", deviceID, "legacyDeviceId", segment.LegacyDeviceID, "error", legacyErr)
			return nil, fmt.Errorf("读取改号前设备数据失败，无法确认完整范围: %w", legacyErr)
		}
		if paged {
			boundary = historyPageStart(part, limit, boundary)
		}
		series = mergeSeries(series, part, segment.SwitchTs)
	}
	if paged {
		for _, points := range series {
			for ts := range points {
				if ts < boundary {
					delete(points, ts)
				}
			}
		}
	}
	if hasSPC {
		groups = append(groups, groupsFromSPC(series["spcvalue"])...)
	}
	ordered := rowStamps(groups, keys, series, hasSPC, 0, desc)
	rows := make([]historyRow, 0, len(ordered))
	outCount := 0
	for _, ts := range ordered {
		values := map[string]string{}
		for _, key := range keys {
			if value := series[key][ts]; value != "" {
				values[key] = value
			}
		}
		if hasSPC {
			for _, item := range parseSPC(series["spcvalue"][ts]) {
				if item.Name == "" {
					continue
				}
				values["spc::"+item.Name] = item.Value
				values["spcU::"+item.Name] = item.UCL
				values["spcL::"+item.Name] = item.LCL
			}
		}
		flags := map[string]string{}
		for _, group := range groups {
			flag := evaluateGroup(group, values)
			if flag != "" {
				flags[group.ValueKey] = flag
			}
			if flag == "OUT" {
				outCount++
			}
		}
		rows = append(rows, historyRow{TS: ts, Time: formatTime(ts), Values: values, Flags: flags})
	}

	spcRows, truncated := s.spcRows(series["spcvalue"], desc)
	dataType := "设备数据"
	quality := hasSPC
	for _, group := range groups {
		if group.Source != "plain" {
			quality = true
			break
		}
	}
	if quality {
		dataType = "质量数据"
	}
	metadata := groupMeta(groups, rows)
	judgeable := false
	for _, group := range metadata {
		if group["judgeable"] == true {
			judgeable = true
			break
		}
	}
	result := map[string]any{
		"deviceId": deviceID, "deviceName": nil, "deviceLabel": nil, "deviceType": nil,
		"dataType": dataType, "judgeable": judgeable,
		"startTime": formatTime(from), "endTime": formatTime(to),
		"keys": keys, "groups": metadata, "rows": rows,
		"outCount": outCount, "spcRows": spcRows, "spcTruncated": truncated,
	}
	if paged && boundary > from {
		result["nextEnd"] = boundary - 1
	}
	if device != nil {
		result["deviceName"] = device.Name
		result["deviceLabel"] = device.Label
		result["deviceType"] = device.Type
	}
	return result, nil
}

func (s *HistoryService) Raw(deviceID string, fromParam, toParam *int64, limitParam int, order string) (json.RawMessage, error) {
	to := time.Now().UnixMilli()
	if toParam != nil {
		to = *toParam
	}
	from := to - int64(7*24*time.Hour/time.Millisecond)
	if fromParam != nil {
		from = *fromParam
	}
	if from > to {
		return nil, fmt.Errorf("开始时间不能晚于结束时间")
	}
	limit := resolveHistoryLimit(limitParam)
	desc := !strings.EqualFold(order, "ASC")
	legacy := s.legacySegments(deviceID, from, to)
	keys, err := s.client.TimeseriesKeys(deviceID)
	if err != nil {
		return nil, err
	}
	for _, segment := range legacy {
		legacyKeys, legacyErr := s.client.TimeseriesKeys(segment.LegacyDeviceID)
		if legacyErr != nil {
			slog.Warn("读取改号前设备字段失败，跳过该段原始报文", "deviceId", deviceID, "legacyDeviceId", segment.LegacyDeviceID, "error", legacyErr)
			return nil, fmt.Errorf("读取改号前设备数据失败，无法确认完整范围: %w", legacyErr)
		}
		keys = unionKeys(keys, legacyKeys)
	}
	if len(keys) == 0 {
		return json.RawMessage("{}"), nil
	}
	payload, err := s.readHistoryRaw(deviceID, keys, from, to, limit, order)
	if err != nil {
		return nil, err
	}
	if len(legacy) == 0 {
		return payload, nil
	}
	sources := []rawSource{{Payload: payload}}
	for _, segment := range legacy {
		legacyEnd, ok := legacyRange(from, to, segment.SwitchTs)
		if !ok {
			continue
		}
		part, legacyErr := s.readHistoryRaw(segment.LegacyDeviceID, keys, from, legacyEnd, limit, order)
		if legacyErr != nil {
			slog.Warn("读取改号前设备原始报文失败，跳过该段数据", "deviceId", deviceID, "legacyDeviceId", segment.LegacyDeviceID, "error", legacyErr)
			return nil, fmt.Errorf("读取改号前设备数据失败，无法确认完整范围: %w", legacyErr)
		}
		sources = append(sources, rawSource{Payload: part, Until: segment.SwitchTs})
	}
	if len(sources) == 1 {
		return payload, nil
	}
	return mergeRawPayloads(sources, keys, limit, desc)
}

func (s *HistoryService) Export(deviceID string, from, to *int64, limit int, order, mode, keysParam string, fill bool) (string, string, error) {
	if strings.EqualFold(mode, "raw") {
		payload, err := s.Raw(deviceID, from, to, limit, order)
		if err != nil {
			return "", "", err
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, payload, "", "  "); err != nil {
			pretty.Write(payload)
		}
		device := s.catalog.Find(deviceID)
		name := deviceID
		if device != nil && device.Name != "" {
			name = device.Name
		}
		return fileStem(name) + "_原始报文_" + fileStamp() + ".json", pretty.String(), nil
	}
	data, err := s.Build(deviceID, from, to, limit, order)
	if err != nil {
		return "", "", err
	}
	keys := stringSlice(data["keys"])
	rows := anyRows(data["rows"])
	if fill {
		for i := len(rows) - 2; i >= 0; i-- {
			for _, key := range keys {
				if rows[i].Values[key] == "" && rows[i+1].Values[key] != "" {
					rows[i].Values[key] = rows[i+1].Values[key]
				}
			}
		}
	}
	groupMaps, _ := data["groups"].([]map[string]any)
	groups := make([]HistoryGroup, 0, len(groupMaps))
	for _, item := range groupMaps {
		groups = append(groups, HistoryGroup{
			Name: fmt.Sprint(item["name"]), ValueKey: fmt.Sprint(item["valueKey"]),
			UpperKey: fmt.Sprint(item["upperKey"]), LowerKey: fmt.Sprint(item["lowerKey"]),
			HasLimit: item["hasLimit"] == true, Source: fmt.Sprint(item["source"]),
		})
	}
	records := [][]string{}
	name := deviceID
	if value := data["deviceName"]; value != nil && fmt.Sprint(value) != "" {
		name = fmt.Sprint(value)
	}
	if strings.EqualFold(mode, "spc") {
		records = append(records, []string{"采集时间", "Name", "Value", "Ucl", "Lcl", "判定"})
		spcRows, _ := data["spcRows"].([]SPCRow)
		for _, item := range spcRows {
			judgement := "无有效限值"
			if item.Out {
				judgement = "超限"
			} else if item.Applicable {
				judgement = "正常"
			}
			records = append(records, []string{
				item.Time, item.Name, item.Value, item.UCL, item.LCL, judgement,
			})
		}
		return fileStem(name) + "_spcvalue明细_" + fileStamp() + ".csv", csvText(records), nil
	}
	header := []string{"采集时间"}
	if strings.EqualFold(mode, "all") {
		header = append(header, keys...)
		for _, row := range rows {
			line := []string{row.Time}
			for _, key := range keys {
				line = append(line, row.Values[key])
			}
			records = append(records, line)
		}
	} else {
		wanted := map[string]bool{}
		for _, key := range strings.Split(keysParam, ",") {
			wanted[strings.TrimSpace(key)] = true
		}
		picked := []HistoryGroup{}
		for _, group := range groups {
			if wanted[group.ValueKey] {
				picked = append(picked, group)
			}
		}
		for _, group := range picked {
			header = append(header, group.Name)
			if group.HasLimit {
				header = append(header, group.Name+"_上限", group.Name+"_下限")
			}
		}
		for _, row := range rows {
			line := []string{row.Time}
			for _, group := range picked {
				line = append(line, row.Values[group.ValueKey])
				if group.HasLimit {
					line = append(line, row.Values[group.UpperKey], row.Values[group.LowerKey])
				}
			}
			records = append(records, line)
		}
	}
	records = append([][]string{header}, records...)
	suffix := "历史数据"
	if strings.EqualFold(mode, "all") {
		suffix = "全部字段"
	}
	return fileStem(name) + "_" + suffix + "_" + fileStamp() + ".csv", csvText(records), nil
}

func (s *HistoryService) fetchSeries(deviceID string, keys []string, from, to int64, limit int, desc bool) (map[string]map[int64]string, error) {
	result := map[string]map[int64]string{}
	if len(keys) == 0 {
		return result, nil
	}
	order := "ASC"
	if desc {
		order = "DESC"
	}
	payload, err := s.readHistoryRaw(deviceID, keys, from, to, limit, order)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}
	for _, key := range keys {
		values := map[int64]string{}
		for _, point := range seriesPoints(data, key) {
			values[point.TS] = point.Value
		}
		result[key] = values
	}
	return result, nil
}

func (s *HistoryService) spcRows(series map[int64]string, desc bool) ([]SPCRow, bool) {
	keys := make([]int64, 0, len(series))
	for ts := range series {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool {
		if desc {
			return keys[i] > keys[j]
		}
		return keys[i] < keys[j]
	})
	rows := []SPCRow{}
	for _, ts := range keys {
		for _, item := range parseSPC(series[ts]) {
			if item.Name == "" {
				continue
			}
			value, valueOK := parseNumber(item.Value)
			upper, upperOK := parseNumber(item.UCL)
			lower, lowerOK := parseNumber(item.LCL)
			applicable := valueOK && upperOK && lowerOK && !(upper == 0 && lower == 0)
			out := applicable && (value < math.Min(upper, lower) || value > math.Max(upper, lower))
			rows = append(rows, SPCRow{
				TS: ts, Time: formatTime(ts), Name: item.Name, Value: item.Value,
				UCL: item.UCL, LCL: item.LCL, Applicable: applicable, Out: out,
			})
		}
	}
	return rows, false
}

// Keep every observation timestamp, including standalone status and limit updates.
func rowStamps(groups []HistoryGroup, keys []string, series map[string]map[int64]string, hasSPC bool, limit int, desc bool) []int64 {
	stamps := map[int64]bool{}
	for _, points := range series {
		for ts := range points {
			stamps[ts] = true
		}
	}
	return sortedTimeKeys(stamps, desc)
}

func detectFieldGroups(keys []string) []HistoryGroup {
	values := map[string]string{}
	uppers := map[string]string{}
	lowers := map[string]string{}
	for _, key := range keys {
		if base, ok := strings.CutSuffix(key, "测量值"); ok {
			if _, exists := values[base]; !exists {
				values[base] = key
			}
		} else if base, ok := strings.CutSuffix(key, "上限"); ok {
			if _, exists := uppers[base]; !exists {
				uppers[base] = key
			}
		} else if base, ok := strings.CutSuffix(key, "下限"); ok {
			if _, exists := lowers[base]; !exists {
				lowers[base] = key
			}
		}
	}
	groups := []HistoryGroup{}
	consumed := map[string]bool{}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.TrimSpace(name) == "" || junkName.MatchString(name) {
			continue
		}
		upper, lower := uppers[name], lowers[name]
		hasLimit := upper != "" && lower != "" && !strings.Contains(name, "设定值")
		groups = append(groups, HistoryGroup{Name: name, ValueKey: values[name], UpperKey: upper, LowerKey: lower, HasLimit: hasLimit, Source: "field"})
		consumed[values[name]] = true
		consumed[upper] = true
		consumed[lower] = true
	}
	for _, key := range keys {
		if key != "spcvalue" && !consumed[key] {
			groups = append(groups, HistoryGroup{Name: key, ValueKey: key, Source: "plain"})
		}
	}
	return groups
}

func groupsFromSPC(series map[int64]string) []HistoryGroup {
	names := map[string]bool{}
	for _, raw := range series {
		for _, item := range parseSPC(raw) {
			if item.Name != "" && !junkName.MatchString(item.Name) {
				names[item.Name] = true
			}
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)
	groups := make([]HistoryGroup, 0, len(ordered))
	for _, name := range ordered {
		groups = append(groups, HistoryGroup{
			Name: name, ValueKey: "spc::" + name, UpperKey: "spcU::" + name,
			LowerKey: "spcL::" + name, HasLimit: true, Source: "spcvalue",
		})
	}
	return groups
}

func parseSPC(raw string) []SPCItem {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var items []SPCItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

func groupMeta(groups []HistoryGroup, rows []historyRow) []map[string]any {
	result := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		booleanCheck := isBooleanGroup(group.Name)
		applicable := false
		for _, row := range rows {
			upper, upperOK := parseNumber(row.Values[group.UpperKey])
			lower, lowerOK := parseNumber(row.Values[group.LowerKey])
			if group.HasLimit && upperOK && lowerOK && !(upper == 0 && lower == 0) {
				applicable = true
				break
			}
		}
		result = append(result, map[string]any{
			"name": group.Name, "valueKey": group.ValueKey,
			"upperKey": group.UpperKey, "lowerKey": group.LowerKey,
			"hasLimit": group.HasLimit, "booleanCheck": booleanCheck,
			"limitApplicable": applicable, "judgeable": applicable || booleanCheck,
			"source": group.Source,
		})
	}
	return result
}

func evaluateGroup(group HistoryGroup, values map[string]string) string {
	value, ok := parseNumber(values[group.ValueKey])
	if !ok {
		return ""
	}
	if isBooleanGroup(group.Name) {
		if value == 0 {
			return "OUT"
		}
		return "OK"
	}
	if !group.HasLimit {
		return ""
	}
	upper, upperOK := parseNumber(values[group.UpperKey])
	lower, lowerOK := parseNumber(values[group.LowerKey])
	if !upperOK || !lowerOK || (upper == 0 && lower == 0) {
		return ""
	}
	if value < math.Min(upper, lower) || value > math.Max(upper, lower) {
		return "OUT"
	}
	return "OK"
}

func parseNumber(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return value, err == nil
}

func buildTelemetryQuery(keys []string, start, end int64, agg string, limit int, order string) url.Values {
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
	return query
}

func isBooleanGroup(name string) bool {
	normalized := strings.ToUpper(strings.ReplaceAll(name, " ", ""))
	return strings.Contains(normalized, "OK或NG") ||
		strings.Contains(normalized, "OK/NG") || strings.Contains(normalized, "OKNG")
}

var junkName = regexp.MustCompile(`^[0-9.\-+]+$`)

func resolveHistoryLimit(value int) int {
	return max(value, 0)
}

func sortedTimeKeys(set map[int64]bool, desc bool) []int64 {
	keys := make([]int64, 0, len(set))
	for ts := range set {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool {
		if desc {
			return keys[i] > keys[j]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func stringSlice(value any) []string {
	raw, _ := value.([]string)
	return append([]string(nil), raw...)
}

func anyRows(value any) []historyRow {
	raw, _ := value.([]historyRow)
	return append([]historyRow(nil), raw...)
}

func anySlice(value any) []any {
	raw, _ := value.([]any)
	return raw
}

func csvText(records [][]string) string {
	var buffer bytes.Buffer
	buffer.WriteString("\ufeff")
	writer := csv.NewWriter(&buffer)
	_ = writer.WriteAll(records)
	writer.Flush()
	return buffer.String()
}

func fileStem(name string) string {
	if strings.TrimSpace(name) == "" {
		name = "device"
	}
	replacer := regexp.MustCompile(`[\\/:*?"<>|\s]+`)
	return strings.Trim(replacer.ReplaceAllString(name, "_"), "_")
}

func fileStamp() string {
	location, _ := time.LoadLocation("Asia/Shanghai")
	return time.Now().In(location).Format("20060102_1504")
}
