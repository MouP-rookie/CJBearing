-- 门户接入前的账号应保留首末检入口，客户端和业务权限继续由原有 RBAC 控制。
-- 保留已有授权记录，并跳过管理员曾明确维护过系统授权的账号，避免恢复被撤销的权限。
INSERT INTO portal_system_access (user_id, system_code, enabled, updated_at)
SELECT u.id, 'bearing_inspection', TRUE, NOW()
FROM users u
JOIN roles r ON r.id = u.role_id AND r.deleted_at IS NULL
WHERE u.deleted_at IS NULL
  AND r.code <> 'super_admin'
  AND NOT EXISTS (
    SELECT 1 FROM audit_logs a
    WHERE a.entity_type = 'portal_system_access' AND a.entity_id = u.id AND a.action = 'update'
  )
ON CONFLICT (user_id, system_code) DO NOTHING;
