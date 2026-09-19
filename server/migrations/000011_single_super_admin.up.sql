BEGIN;

CREATE OR REPLACE FUNCTION enforce_single_active_super_admin()
RETURNS trigger AS $$
DECLARE
  super_role_id BIGINT;
BEGIN
  SELECT id INTO super_role_id
  FROM roles
  WHERE code = 'super_admin' AND deleted_at IS NULL
  LIMIT 1;

  IF super_role_id IS NULL OR NEW.role_id <> super_role_id OR NEW.deleted_at IS NOT NULL THEN
    RETURN NEW;
  END IF;

  PERFORM pg_advisory_xact_lock(hashtext('bearing_inspection_single_super_admin'));

  IF EXISTS (
    SELECT 1
    FROM users
    WHERE id <> COALESCE(NEW.id, 0)
      AND role_id = super_role_id
      AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'only one active super administrator is allowed'
      USING ERRCODE = '23505';
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

WITH ranked AS (
  SELECT u.id,
         ROW_NUMBER() OVER (
           ORDER BY CASE WHEN u.account = 'admin' THEN 0 ELSE 1 END, u.id
         ) AS rn
  FROM users u
  JOIN roles r ON r.id = u.role_id
  WHERE r.code = 'super_admin'
    AND r.deleted_at IS NULL
    AND u.deleted_at IS NULL
), duplicates AS (
  SELECT id FROM ranked WHERE rn > 1
)
UPDATE users u
SET role_id = (
      COALESCE(
        (SELECT id FROM roles WHERE code = 'admin' AND deleted_at IS NULL LIMIT 1),
        (SELECT id FROM roles WHERE code = 'normal' AND deleted_at IS NULL LIMIT 1)
      )
    ),
    department_id = COALESCE(
      u.department_id,
      (SELECT id FROM departments WHERE code = 'donggang' AND deleted_at IS NULL LIMIT 1)
    )
FROM duplicates d
WHERE u.id = d.id
  AND EXISTS (SELECT 1 FROM roles WHERE code IN ('admin', 'normal') AND deleted_at IS NULL);

DO $$
BEGIN
  IF (
    SELECT COUNT(*)
    FROM users u
    JOIN roles r ON r.id = u.role_id
    WHERE r.code = 'super_admin'
      AND r.deleted_at IS NULL
      AND u.deleted_at IS NULL
  ) > 1 THEN
    RAISE EXCEPTION 'failed to normalize multiple active super administrators';
  END IF;
END;
$$;

CREATE TRIGGER trg_single_active_super_admin
BEFORE INSERT OR UPDATE OF role_id, deleted_at ON users
FOR EACH ROW EXECUTE FUNCTION enforce_single_active_super_admin();

COMMIT;
