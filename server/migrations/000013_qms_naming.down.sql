BEGIN;

UPDATE permissions
SET code = REPLACE(code, 'qms.', 'hse.'),
    name = REPLACE(name, 'QMS', 'HSE'),
    "group" = REPLACE("group", 'QMS', 'HSE')
WHERE code LIKE 'qms.%';

INSERT INTO portal_systems
  (code, name, description, entry_path, entry_port, health_path, accent, icon, enabled, sort_order)
SELECT
  'hse_iot',
  'HSE 环境监测系统',
  '淬火线油烟治理、温湿度环境与设备历史数据。',
  entry_path,
  entry_port,
  health_path,
  accent,
  icon,
  enabled,
  sort_order
FROM portal_systems
WHERE code = 'qms_iot'
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
SELECT user_id, 'hse_iot', enabled, updated_by, updated_at
FROM portal_system_access
WHERE system_code = 'qms_iot'
ON CONFLICT (user_id, system_code) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  updated_by = EXCLUDED.updated_by,
  updated_at = EXCLUDED.updated_at;

DELETE FROM portal_system_access WHERE system_code = 'qms_iot';
DELETE FROM portal_systems WHERE code = 'qms_iot';

COMMIT;
