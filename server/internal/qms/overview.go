// 文件说明：环境监测总览聚合与时间参数解析。

package qms

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bearing-inspection/internal/config"
)

type OverviewQuery struct {
	Limit     int
	Refresh   bool
	Days      *int
	Start     *int64
	End       *int64
	DataDays  *int
	DataStart *int64
	DataEnd   *int64
}

func ParseQuery(values map[string][]string) OverviewQuery {
	query := OverviewQuery{Limit: 10}
	if raw := firstQuery(values, "limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			query.Limit = parsed
		}
	}
	query.Limit = min(max(query.Limit, 1), 2000)
	query.Refresh = strings.EqualFold(firstQuery(values, "refresh"), "true")
	query.Days = optionalInt(firstQuery(values, "days"))
	query.DataDays = optionalInt(firstQuery(values, "dataDays"))
	query.Start = ParseTime(firstQuery(values, "start"))
	query.End = ParseTime(firstQuery(values, "end"))
	query.DataStart = ParseTime(firstQuery(values, "dataStart"))
	query.DataEnd = ParseTime(firstQuery(values, "dataEnd"))
	return query
}

func Overview(cfg *config.Config, telemetry *TelemetryService, query OverviewQuery) (map[string]any, error) {
	now := time.Now().UnixMilli()
	powerFrom := query.Start
	powerTo := query.End
	if powerFrom == nil && powerTo == nil && query.Days != nil && *query.Days > 0 {
		value := now - int64(*query.Days)*int64(24*time.Hour/time.Millisecond)
		powerFrom = &value
	}
	dataFrom := query.DataStart
	dataTo := query.DataEnd
	if dataFrom == nil && dataTo == nil && query.DataDays != nil && *query.DataDays > 0 {
		value := now - int64(*query.DataDays)*int64(24*time.Hour/time.Millisecond)
		dataFrom = &value
	}
	dataLimit := query.Limit
	if dataFrom != nil || dataTo != nil {
		dataLimit = 2000
	}
	powerRows, err := telemetry.PowerRecords(query.Refresh, powerFrom, powerTo)
	if err != nil {
		return nil, err
	}
	pressureRows, err := telemetry.PressureRows(dataLimit, dataFrom, dataTo)
	if err != nil {
		return nil, err
	}
	thRows, err := telemetry.THRows(dataLimit, dataFrom, dataTo)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"generatedAt": formatTime(now),
		"source":      cfg.IOTBaseURL,
		"power": block(
			cfg.IOTPressureID, cfg.IOTPressureName, cfg.IOTPressureLabel, powerRows,
		),
		"pressure": block(
			cfg.IOTPressureID, cfg.IOTPressureName, cfg.IOTPressureLabel, pressureRows,
		),
		"th": block(
			cfg.IOTTHID, cfg.IOTTHName, cfg.IOTTHLabel, thRows,
		),
	}, nil
}

func block(deviceID, name, label string, rows any) map[string]any {
	return map[string]any{
		"deviceId": deviceID, "deviceName": name, "deviceLabel": label, "rows": rows,
	}
}

func ParseTime(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.ReplaceAll(raw, " ", "T")
	layouts := []string{"2006-01-02T15:04", "2006-01-02T15:04:05"}
	location, _ := time.LoadLocation("Asia/Shanghai")
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, location); err == nil {
			value := parsed.UnixMilli()
			return &value
		}
	}
	return nil
}

func firstQuery(values map[string][]string, key string) string {
	if list := values[key]; len(list) > 0 {
		return list[0]
	}
	return ""
}

func optionalInt(raw string) *int {
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func asBadRequest(err error) (int, map[string]any) {
	return http.StatusBadRequest, map[string]any{"error": fmt.Sprint(err)}
}
