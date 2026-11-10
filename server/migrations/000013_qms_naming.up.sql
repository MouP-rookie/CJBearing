BEGIN;

-- 将早期 HSE 命名迁移为实际业务域 QMS；权限 ID 不变，因此角色授权自动保留。
UPDATE permissions
SET code = REPLACE(code, 'hse.', 'qms.'),
    name = REPLACE(REPLACE(name, 'HSE', 'QMS'), '环境监测', '质量管理'),
    "group" = REPLACE("group", 'HSE', 'QMS')
WHERE code LIKE 'hse.%';

INSERT INTO portal_systems
  (code, name, description, entry_path, entry_port, health_path, accent, icon, enabled, sort_order)
SELECT
  'qms_iot',
  'QMS 质量管理系统',
  '质量过程、环境设备、历史数据与检测结果统一查询。',
  entry_path,
  entry_port,
  health_path,
  accent,
  icon,
  enabled,
  sort_order
FROM portal_systems
WHERE code = 'hse_iot'
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  entry_path = EXCLUDED.entry_path,
  entry_port = EXCLUDED.entry_port,
  health_path = EXCLUDED.health_path,
  accent = EXCLUDED.accent,
  icon = EXCLUDED.icon,
  enabled = EXCLUDED.enabled,
  sort_order = EXCLUDED.sort_order,
  updated_at = NOW();

INSERT INTO portal_system_access (user_id, system_code, enabled, updated_by, updated_at)
SELECT user_id, 'qms_iot', enabled, updated_by, updated_at
FROM portal_system_access
WHERE system_code = 'hse_iot'
ON CONFLICT (user_id, system_code) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  updated_by = EXCLUDED.updated_by,
  updated_at = EXCLUDED.updated_at;

DELETE FROM portal_system_access WHERE system_code = 'hse_iot';
DELETE FROM portal_systems WHERE code = 'hse_iot';

COMMIT;
