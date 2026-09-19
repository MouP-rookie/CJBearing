CREATE TABLE IF NOT EXISTS submissions (
  id BIGSERIAL PRIMARY KEY,
  template_id BIGINT NOT NULL REFERENCES form_templates(id),
  version_id BIGINT NOT NULL REFERENCES form_versions(id),
  line_id BIGINT NOT NULL REFERENCES lines(id),
  business_date DATE NOT NULL,
  shift_plan_item_id BIGINT NOT NULL REFERENCES shift_plan_items(id),
  shift_name_snapshot VARCHAR(64) NOT NULL,
  shift_start_snapshot VARCHAR(5) NOT NULL,
  shift_end_snapshot VARCHAR(5) NOT NULL,
  repeat_reason TEXT NOT NULL DEFAULT '',
  source VARCHAR(32) NOT NULL DEFAULT 'online',
  client_time TIMESTAMPTZ,
  created_by BIGINT NOT NULL REFERENCES users(id),
  submitted_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_submissions_line_date ON submissions(line_id, business_date);
CREATE INDEX IF NOT EXISTS idx_submissions_template_version ON submissions(template_id, version_id);
CREATE INDEX IF NOT EXISTS idx_submissions_deleted_at ON submissions(deleted_at);

CREATE TABLE IF NOT EXISTS submission_rows (
  id BIGSERIAL PRIMARY KEY,
  submission_id BIGINT NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  row_no INTEGER NOT NULL,
  operator_id BIGINT NOT NULL REFERENCES users(id),
  operated_at TIMESTAMPTZ NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  void_reason TEXT NOT NULL DEFAULT '',
  voided_by BIGINT REFERENCES users(id),
  voided_at TIMESTAMPTZ,
  restored_by BIGINT REFERENCES users(id),
  restored_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE(submission_id, row_no)
);
CREATE INDEX IF NOT EXISTS idx_submission_rows_status ON submission_rows(status);
CREATE INDEX IF NOT EXISTS idx_submission_rows_deleted_at ON submission_rows(deleted_at);

CREATE TABLE IF NOT EXISTS field_values (
  id BIGSERIAL PRIMARY KEY,
  submission_id BIGINT NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  row_id BIGINT REFERENCES submission_rows(id) ON DELETE CASCADE,
  field_key VARCHAR(96) NOT NULL,
  value_type VARCHAR(32) NOT NULL,
  value_text TEXT NOT NULL DEFAULT '',
  value_number DOUBLE PRECISION,
  repeat_index INTEGER NOT NULL DEFAULT 0,
  source VARCHAR(32) NOT NULL DEFAULT 'manual',
  created_by BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_field_values_submission ON field_values(submission_id);
CREATE INDEX IF NOT EXISTS idx_field_values_row ON field_values(row_id);
CREATE INDEX IF NOT EXISTS idx_field_values_key ON field_values(field_key);
