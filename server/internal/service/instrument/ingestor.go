package instrument

import (
	"encoding/json"

	"bearing-inspection/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Ingestor interface {
	Ingest(topic, deviceID string, departmentID uint64, raw json.RawMessage) (*model.InstrumentMessage, error)
}

type DBStore struct{ db *gorm.DB }

func NewDBStore(db *gorm.DB) *DBStore { return &DBStore{db: db} }

func (s *DBStore) Ingest(topic, deviceID string, departmentID uint64, raw json.RawMessage) (*model.InstrumentMessage, error) {
	msg := &model.InstrumentMessage{Topic: topic, DeviceID: deviceID, RawJSON: datatypes.JSON(raw), ReceivedAt: timeNow()}
	if departmentID > 0 {
		msg.DepartmentID = &departmentID
	}
	if err := s.db.Create(msg).Error; err != nil {
		return nil, err
	}
	return msg, nil
}
