ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_code_key;
DROP INDEX IF EXISTS ux_roles_code_active;
CREATE UNIQUE INDEX ux_roles_code_active ON roles(code) WHERE deleted_at IS NULL;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_account_key;
DROP INDEX IF EXISTS ux_users_account_active;
CREATE UNIQUE INDEX ux_users_account_active ON users(account) WHERE deleted_at IS NULL;

ALTER TABLE lines DROP CONSTRAINT IF EXISTS lines_code_key;
DROP INDEX IF EXISTS ux_lines_code_active;
CREATE UNIQUE INDEX ux_lines_code_active ON lines(code) WHERE deleted_at IS NULL;

ALTER TABLE form_templates DROP CONSTRAINT IF EXISTS form_templates_code_key;
DROP INDEX IF EXISTS ux_form_templates_code_active;
CREATE UNIQUE INDEX ux_form_templates_code_active ON form_templates(code) WHERE deleted_at IS NULL;