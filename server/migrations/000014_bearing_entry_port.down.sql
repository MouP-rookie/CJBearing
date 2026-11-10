UPDATE portal_systems
SET entry_port = 8080, updated_at = NOW()
WHERE code = 'bearing_inspection' AND entry_port = 8090;
