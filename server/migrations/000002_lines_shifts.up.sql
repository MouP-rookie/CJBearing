CREATE TABLE IF NOT EXISTS lines (
  id BIGSERIAL PRIMARY KEY,
  parent_id BIGINT REFERENCES lines(id),
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  level INTEGER NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_lines_parent_id ON lines(parent_id);
CREATE INDEX IF NOT EXISTS idx_lines_deleted_at ON lines(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_lines_code_active ON lines(code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS shift_plans (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  effective_at TIMESTAMPTZ NOT NULL,
  created_by BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_shift_plans_effective_at ON shift_plans(effective_at);
CREATE INDEX IF NOT EXISTS idx_shift_plans_deleted_at ON shift_plans(deleted_at);

CREATE TABLE IF NOT EXISTS shift_plan_items (
  id BIGSERIAL PRIMARY KEY,
  plan_id BIGINT NOT NULL REFERENCES shift_plans(id) ON DELETE CASCADE,
  name VARCHAR(64) NOT NULL,
  start_time VARCHAR(5) NOT NULL,
  end_time VARCHAR(5) NOT NULL,
  cross_midnight BOOLEAN NOT NULL DEFAULT FALSE,
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_shift_plan_items_plan_id ON shift_plan_items(plan_id);
