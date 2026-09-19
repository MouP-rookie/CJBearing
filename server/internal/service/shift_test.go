package service

import (
	"testing"
	"time"

	"bearing-inspection/internal/model"
)

func TestCalculateShiftDayShift(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	plan := model.ShiftPlan{Items: []model.ShiftPlanItem{{ID: 1, Name: "白班", StartTime: "07:00", EndTime: "19:00"}, {ID: 2, Name: "中班", StartTime: "19:00", EndTime: "07:00", CrossMidnight: true}}}
	got, err := CalculateShift(plan, time.Date(2026, 9, 17, 10, 30, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if got.Item.ID != 1 {
		t.Fatalf("want day shift, got %+v", got.Item)
	}
	if got.BusinessDate.Format("2006-01-02") != "2026-09-17" {
		t.Fatalf("unexpected business date: %s", got.BusinessDate)
	}
}

func TestCalculateShiftCrossMidnightUsesStartDate(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	plan := model.ShiftPlan{Items: []model.ShiftPlanItem{{ID: 2, Name: "中班", StartTime: "19:00", EndTime: "07:00", CrossMidnight: true}}}
	got, err := CalculateShift(plan, time.Date(2026, 9, 18, 2, 0, 0, 0, loc))
	if err != nil {
		t.Fatal(err)
	}
	if got.BusinessDate.Format("2006-01-02") != "2026-09-17" {
		t.Fatalf("expected start date, got %s", got.BusinessDate)
	}
}
