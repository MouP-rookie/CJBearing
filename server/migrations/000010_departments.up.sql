BEGIN;

CREATE TABLE IF NOT EXISTS departments (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_departments_code_active ON departments(code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_departments_deleted_at ON departments(deleted_at);

INSERT INTO departments (code, name, enabled)
VALUES ('donggang', '东港分厂', TRUE)
ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;

ALTER TABLE users ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE lines ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE products ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE shift_plans ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE form_templates ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE backup_records ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);
ALTER TABLE backup_records ADD COLUMN IF NOT EXISTS kind VARCHAR(32) NOT NULL DEFAULT 'full';
ALTER TABLE instrument_messages ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id);

UPDATE users
SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL)
WHERE department_id IS NULL
  AND role_id NOT IN (SELECT id FROM roles WHERE code = 'super_admin' AND deleted_at IS NULL);

UPDATE lines SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL) WHERE department_id IS NULL;
UPDATE products SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL) WHERE department_id IS NULL;
UPDATE shift_plans SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL) WHERE department_id IS NULL;
UPDATE form_templates SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL) WHERE department_id IS NULL;
UPDATE submissions SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL) WHERE department_id IS NULL;
UPDATE audit_logs SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL) WHERE department_id IS NULL;
UPDATE instrument_messages SET department_id = (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL) WHERE department_id IS NULL;

ALTER TABLE lines ALTER COLUMN department_id SET NOT NULL;
ALTER TABLE products ALTER COLUMN department_id SET NOT NULL;
ALTER TABLE shift_plans ALTER COLUMN department_id SET NOT NULL;
ALTER TABLE form_templates ALTER COLUMN department_id SET NOT NULL;
ALTER TABLE submissions ALTER COLUMN department_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_department_id ON users(department_id);
CREATE INDEX IF NOT EXISTS idx_lines_department_id ON lines(department_id);
CREATE INDEX IF NOT EXISTS idx_products_department_id ON products(department_id);
CREATE INDEX IF NOT EXISTS idx_shift_plans_department_id ON shift_plans(department_id);
CREATE INDEX IF NOT EXISTS idx_form_templates_department_id ON form_templates(department_id);
CREATE INDEX IF NOT EXISTS idx_submissions_department_id ON submissions(department_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_department_id ON audit_logs(department_id);
CREATE INDEX IF NOT EXISTS idx_backup_records_department_id ON backup_records(department_id);
CREATE INDEX IF NOT EXISTS idx_backup_records_kind ON backup_records(kind);
CREATE INDEX IF NOT EXISTS idx_instrument_messages_department_id ON instrument_messages(department_id);

DROP INDEX IF EXISTS ux_lines_code_active;
DROP INDEX IF EXISTS ux_products_code_active;
DROP INDEX IF EXISTS ux_form_templates_code_active;
CREATE UNIQUE INDEX ux_lines_department_code_active ON lines(department_id, code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX ux_products_department_code_active ON products(department_id, code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX ux_form_templates_department_code_active ON form_templates(department_id, code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS department_settings (
  department_id BIGINT NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
  key VARCHAR(64) NOT NULL,
  value TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (department_id, key)
);

INSERT INTO department_settings (department_id, key, value)
SELECT d.id, s.key, s.value
FROM departments d
JOIN app_settings s ON s.key IN ('brand_logo', 'submission_void_reasons')
WHERE d.code = 'donggang' AND d.deleted_at IS NULL
ON CONFLICT (department_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();

COMMIT;
