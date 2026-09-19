package backup

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"bearing-inspection/internal/model"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	pgDump string
	dir    string
}

func New(db *gorm.DB, pgDump, dir string) *Service {
	return &Service{db: db, pgDump: pgDump, dir: dir}
}

func (s *Service) Create(databaseURL string, actorID uint64) (*model.BackupRecord, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("dg_inspection_%s.dump", time.Now().Format("20060102_150405"))
	path := filepath.Join(s.dir, name)
	backupURL := databaseURL
	if u, err := url.Parse(databaseURL); err == nil {
		q := u.Query()
		q.Del("timezone")
		q.Del("TimeZone")
		u.RawQuery = q.Encode()
		backupURL = u.String()
	}
	cmd := exec.Command(s.pgDump, "--format=custom", "--file", path, "--dbname", backupURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pg_dump: %w: %s", err, string(out))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	rec := &model.BackupRecord{FileName: name, FilePath: path, FileSize: info.Size(), CreatedBy: actorID}
	if err := s.db.Create(rec).Error; err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *Service) List() ([]model.BackupRecord, error) {
	var out []model.BackupRecord
	return out, s.db.Order("created_at desc").Find(&out).Error
}

func (s *Service) Get(id uint64) (*model.BackupRecord, error) {
	var out model.BackupRecord
	if err := s.db.First(&out, id).Error; err != nil {
		return nil, err
	}
	return &out, nil
}
