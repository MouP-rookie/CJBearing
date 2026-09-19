package service

import (
	"errors"
	"fmt"
	"time"

	"bearing-inspection/internal/model"
	"gorm.io/gorm"
)

var ErrShiftNotFound = errors.New("shift not found")

type ShiftSelection struct {
	Plan         model.ShiftPlan
	Item         model.ShiftPlanItem
	BusinessDate time.Time
}

func ActiveShiftPlan(db *gorm.DB) (*model.ShiftPlan, error) {
	var plan model.ShiftPlan
	err := db.Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc, id asc") }).
		Where("effective_at <= ?", time.Now()).Order("effective_at desc").First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func CalculateShift(plan model.ShiftPlan, at time.Time) (*ShiftSelection, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	at = at.In(loc)
	for _, item := range plan.Items {
		start, err := parseClock(item.StartTime)
		if err != nil {
			continue
		}
		end, err := parseClock(item.EndTime)
		if err != nil {
			continue
		}
		startAt := time.Date(at.Year(), at.Month(), at.Day(), start.Hour(), start.Minute(), 0, 0, loc)
		endAt := time.Date(at.Year(), at.Month(), at.Day(), end.Hour(), end.Minute(), 0, 0, loc)
		date := startAt
		if item.CrossMidnight || startAt.After(endAt) || startAt.Equal(endAt) {
			if !at.Before(startAt) {
				endAt = endAt.Add(24 * time.Hour)
			} else if at.Before(endAt) {
				startAt = startAt.Add(-24 * time.Hour)
				date = startAt
			} else {
				continue
			}
		} else if at.Before(startAt) || !at.Before(endAt) {
			continue
		}
		if !at.Before(startAt) && at.Before(endAt) {
			return &ShiftSelection{Plan: plan, Item: item, BusinessDate: beginOfDay(date, loc)}, nil
		}
	}
	return nil, ErrShiftNotFound
}

func ParseBusinessTime(value string) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	return time.ParseInLocation("2006-01-02", value, loc)
}

func beginOfDay(t time.Time, loc *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func parseClock(value string) (time.Time, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid clock %q: %w", value, err)
	}
	return t, nil
}
