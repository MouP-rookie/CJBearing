BEGIN;

-- 门户系统注册表让后续系统通过数据和配置接入，而不是修改门户核心代码。
CREATE TABLE IF NOT EXISTS portal_systems (
  code VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  entry_path VARCHAR(255) NOT NULL,
  entry_port INTEGER NOT NULL,
  health_path VARCHAR(255) NOT NULL DEFAULT '/health',
  accent VARCHAR(32) NOT NULL DEFAULT '#17647d',
  icon VARCHAR(32) NOT NULL DEFAULT 'system',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INTEGER NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 这一层只控制“能否查看某个系统”，系统内部的功能权限仍由各子系统 RBAC 校验。
CREATE TABLE IF NOT EXISTS portal_system_access (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  system_code VARCHAR(64) NOT NULL REFERENCES portal_systems(code) ON DELETE CASCADE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, system_code)
);

CREATE INDEX IF NOT EXISTS idx_portal_system_access_code
  ON portal_system_access(system_code, enabled);

INSERT INTO portal_systems
  (code, name, description, entry_path, entry_port, health_path, accent, icon, enabled, sort_order)
VALUES
  (
    'bearing_inspection',
    '轴承首末检系统',
    '首件、末件检验以及报表、登记和过程追溯。',
    '/pc/',
    8090,
    '/health',
    '#17647d',
    'inspection',
    TRUE,
    10
  ),
  (
    'qms_iot',
    'QMS 质量管理系统',
    '质量过程、环境设备、历史数据与检测结果统一查询。',
    '/',
    8081,
    '/api/health',
    '#2e8b65',
    'environment',
    TRUE,
    20
  )
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  entry_path = EXCLUDED.entry_path,
  entry_port = EXCLUDED.entry_port,
  health_path = EXCLUDED.health_path,
  accent = EXCLUDED.accent,
  icon = EXCLUDED.icon,
  sort_order = EXCLUDED.sort_order,
  updated_at = NOW();

-- 默认超级管理员自动拥有所有当前和后续注册系统的查看权限。
INSERT INTO portal_system_access (user_id, system_code, enabled)
SELECT u.id, ps.code, TRUE
FROM users u
JOIN roles r ON r.id = u.role_id
CROSS JOIN portal_systems ps
WHERE r.code = 'super_admin'
  AND u.deleted_at IS NULL
  AND r.deleted_at IS NULL
ON CONFLICT (user_id, system_code) DO UPDATE SET enabled = TRUE, updated_at = NOW();

COMMIT;
