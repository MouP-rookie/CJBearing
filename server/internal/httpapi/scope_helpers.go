package httpapi

import (
	"errors"
	"sort"
	"strings"

	"bearing-inspection/internal/model"
)

func sortLinesByCode(lines []model.Line) {
	sort.SliceStable(lines, func(i, j int) bool {
		if cmp := compareLineCode(lines[i].Code, lines[j].Code); cmp != 0 {
			return cmp < 0
		}
		return lines[i].ID < lines[j].ID
	})
}

func sortLinesByDepartmentAndCode(lines []model.Line) {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].DepartmentID != lines[j].DepartmentID {
			return lines[i].DepartmentID < lines[j].DepartmentID
		}
		if cmp := compareLineCode(lines[i].Code, lines[j].Code); cmp != 0 {
			return cmp < 0
		}
		return lines[i].ID < lines[j].ID
	})
}

func compareLineCode(a, b string) int {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	for ai, bi := 0, 0; ai < len(a) && bi < len(b); {
		if isASCIIDigit(a[ai]) && isASCIIDigit(b[bi]) {
			aStart, bStart := ai, bi
			for ai < len(a) && isASCIIDigit(a[ai]) {
				ai++
			}
			for bi < len(b) && isASCIIDigit(b[bi]) {
				bi++
			}
			aNum := strings.TrimLeft(a[aStart:ai], "0")
			bNum := strings.TrimLeft(b[bStart:bi], "0")
			if aNum == "" {
				aNum = "0"
			}
			if bNum == "" {
				bNum = "0"
			}
			if len(aNum) != len(bNum) {
				if len(aNum) < len(bNum) {
					return -1
				}
				return 1
			}
			if aNum != bNum {
				if aNum < bNum {
					return -1
				}
				return 1
			}
			continue
		}
		if a[ai] != b[bi] {
			if a[ai] < b[bi] {
				return -1
			}
			return 1
		}
		ai++
		bi++
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func (s *Server) ensureLinesInDepartment(lineIDs []uint64, departmentID uint64) error {
	if len(lineIDs) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(lineIDs))
	unique := make([]uint64, 0, len(lineIDs))
	for _, id := range lineIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	var count int64
	if err := s.db.Model(&model.Line{}).Where("id IN ? AND department_id = ?", unique, departmentID).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(unique)) {
		return errors.New("line scope mismatch")
	}
	return nil
}

func (s *Server) ensureUsersInDepartment(userIDs []uint64, departmentID uint64) error {
	if len(userIDs) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(userIDs))
	unique := make([]uint64, 0, len(userIDs))
	for _, id := range userIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil
	}
	var count int64
	if err := s.db.Model(&model.User{}).Where("id IN ? AND department_id = ?", unique, departmentID).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(unique)) {
		return errors.New("user scope mismatch")
	}
	return nil
}

func (s *Server) canLineInScope(scope *departmentScope, permission string, lineID uint64) bool {
	if scope == nil || !s.canLine(scope.User.ID, permission, lineID) {
		return false
	}
	if scope.AllDepartments {
		return true
	}
	var line model.Line
	if err := s.db.First(&line, lineID).Error; err != nil {
		return false
	}
	return line.DepartmentID == scope.DepartmentID
}
