CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  entity_type VARCHAR(64) NOT NULL,
  entity_id BIGINT NOT NULL,
  action VARCHAR(64) NOT NULL,
  before_json JSONB,
  after_json JSONB,
  actor_id BIGINT NOT NULL DEFAULT 0,
  ip VARCHAR(64) NOT NULL DEFAULT '',
  request_id VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

CREATE TABLE IF NOT EXISTS instrument_messages (
  id BIGSERIAL PRIMARY KEY,
  topic VARCHAR(255) NOT NULL DEFAULT '',
  device_id VARCHAR(128) NOT NULL DEFAULT '',
  raw_json JSONB NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  processed BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_instrument_messages_device ON instrument_messages(device_id);
CREATE INDEX IF NOT EXISTS idx_instrument_messages_received ON instrument_messages(received_at);

CREATE TABLE IF NOT EXISTS backup_records (
  id BIGSERIAL PRIMARY KEY,
  file_name VARCHAR(255) NOT NULL,
  file_path VARCHAR(512) NOT NULL,
  file_size BIGINT NOT NULL DEFAULT 0,
  created_by BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
