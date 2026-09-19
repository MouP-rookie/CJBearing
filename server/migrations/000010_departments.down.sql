BEGIN;

DROP TABLE IF EXISTS department_settings;

DROP INDEX IF EXISTS ux_lines_department_code_active;
DROP INDEX IF EXISTS ux_products_department_code_active;
DROP INDEX IF EXISTS ux_form_templates_department_code_active;
CREATE UNIQUE INDEX IF NOT EXISTS ux_lines_code_active ON lines(code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_products_code_active ON products(code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS ux_form_templates_code_active ON form_templates(code) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS idx_users_department_id;
DROP INDEX IF EXISTS idx_lines_department_id;
DROP INDEX IF EXISTS idx_products_department_id;
DROP INDEX IF EXISTS idx_shift_plans_department_id;
DROP INDEX IF EXISTS idx_form_templates_department_id;
DROP INDEX IF EXISTS idx_submissions_department_id;
DROP INDEX IF EXISTS idx_audit_logs_department_id;
DROP INDEX IF EXISTS idx_backup_records_department_id;
DROP INDEX IF EXISTS idx_backup_records_kind;
DROP INDEX IF EXISTS idx_instrument_messages_department_id;

ALTER TABLE instrument_messages DROP COLUMN IF EXISTS department_id;
ALTER TABLE backup_records DROP COLUMN IF EXISTS kind;
ALTER TABLE backup_records DROP COLUMN IF EXISTS department_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS department_id;
ALTER TABLE submissions DROP COLUMN IF EXISTS department_id;
ALTER TABLE form_templates DROP COLUMN IF EXISTS department_id;
ALTER TABLE shift_plans DROP COLUMN IF EXISTS department_id;
ALTER TABLE products DROP COLUMN IF EXISTS department_id;
ALTER TABLE lines DROP COLUMN IF EXISTS department_id;
ALTER TABLE users DROP COLUMN IF EXISTS department_id;

DROP TABLE IF EXISTS departments;

COMMIT;
