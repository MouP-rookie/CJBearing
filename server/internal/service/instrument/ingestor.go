// 文件说明：仪表数据接入接口及数据库实现，V1 预留 MQTT 扩展点。

package instrument

import (
	"encoding/json"

	"bearing-inspection/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Ingestor 抽象仪表数据写入，后续 MQTT 消费者只需实现该接口。
type Ingestor interface {
	Ingest(topic, deviceID string, departmentID uint64, raw json.RawMessage) (*model.InstrumentMessage, error)
}

type DBStore struct{ db *gorm.DB }

func NewDBStore(db *gorm.DB) *DBStore { return &DBStore{db: db} }

// Ingest 保存原始 JSON 报文，不对仪表协议做业务解析。
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
