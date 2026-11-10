-- 修正已部署数据库中遗留的首末检入口；保留管理员配置的其他端口。
UPDATE portal_systems
SET entry_port = 8090, updated_at = NOW()
WHERE code = 'bearing_inspection' AND entry_port = 8080;
