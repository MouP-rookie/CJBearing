CREATE TABLE IF NOT EXISTS form_templates (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(96) NOT NULL,
  name VARCHAR(128) NOT NULL,
  kind VARCHAR(32) NOT NULL,
  paired_template_id BIGINT REFERENCES form_templates(id),
  created_by BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_form_templates_kind ON form_templates(kind);
CREATE INDEX IF NOT EXISTS idx_form_templates_deleted_at ON form_templates(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_form_templates_code_active ON form_templates(code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS form_versions (
  id BIGSERIAL PRIMARY KEY,
  template_id BIGINT NOT NULL REFERENCES form_templates(id) ON DELETE CASCADE,
  version_no INTEGER NOT NULL,
  schema JSONB NOT NULL DEFAULT '{"fields":[]}'::jsonb,
  status VARCHAR(32) NOT NULL,
  published_at TIMESTAMPTZ,
  created_by BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(template_id, version_no)
);
CREATE INDEX IF NOT EXISTS idx_form_versions_status ON form_versions(status);

CREATE TABLE IF NOT EXISTS line_form_assignments (
  id BIGSERIAL PRIMARY KEY,
  line_id BIGINT NOT NULL REFERENCES lines(id),
  template_id BIGINT NOT NULL REFERENCES form_templates(id),
  active_version_id BIGINT NOT NULL REFERENCES form_versions(id),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(line_id, template_id)
);
CREATE INDEX IF NOT EXISTS idx_line_form_assignments_active_version ON line_form_assignments(active_version_id);
