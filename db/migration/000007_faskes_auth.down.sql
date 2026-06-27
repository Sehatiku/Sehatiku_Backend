-- 000007_faskes_auth.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_faskes_username;

ALTER TABLE faskes
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS phone_number;

COMMIT;
