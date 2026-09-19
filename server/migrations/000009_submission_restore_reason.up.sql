ALTER TABLE submission_rows
  ADD COLUMN IF NOT EXISTS restore_reason TEXT NOT NULL DEFAULT '';
