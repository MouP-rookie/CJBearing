CREATE TABLE IF NOT EXISTS roles (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  is_system BOOLEAN NOT NULL DEFAULT FALSE,
  allow_pc BOOLEAN NOT NULL DEFAULT FALSE,
  allow_pda BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_roles_code_active ON roles(code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS permissions (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(96) NOT NULL UNIQUE,
  name VARCHAR(128) NOT NULL,
  "group" VARCHAR(64) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_permissions_group ON permissions("group");

CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  account VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  role_id BIGINT NOT NULL REFERENCES roles(id),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_users_account_active ON users(account) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY(role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS role_line_scopes (
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  line_id BIGINT NOT NULL,
  PRIMARY KEY(role_id, line_id)
);
