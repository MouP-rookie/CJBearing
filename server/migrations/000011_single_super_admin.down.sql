BEGIN;

DROP TRIGGER IF EXISTS trg_single_active_super_admin ON users;
DROP FUNCTION IF EXISTS enforce_single_active_super_admin();

COMMIT;
