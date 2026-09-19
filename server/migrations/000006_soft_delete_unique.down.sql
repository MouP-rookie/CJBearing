DROP INDEX IF EXISTS ux_form_templates_code_active;
ALTER TABLE form_templates ADD CONSTRAINT form_templates_code_key UNIQUE(code);

DROP INDEX IF EXISTS ux_lines_code_active;
ALTER TABLE lines ADD CONSTRAINT lines_code_key UNIQUE(code);

DROP INDEX IF EXISTS ux_users_account_active;
ALTER TABLE users ADD CONSTRAINT users_account_key UNIQUE(account);

DROP INDEX IF EXISTS ux_roles_code_active;
ALTER TABLE roles ADD CONSTRAINT roles_code_key UNIQUE(code);