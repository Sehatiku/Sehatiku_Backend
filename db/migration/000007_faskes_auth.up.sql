-- 000007_faskes_auth.up.sql
-- Tambah credential (username, password_hash, phone_number) ke tabel faskes
-- supaya faskes bisa register dan login secara mandiri.

BEGIN;

ALTER TABLE faskes
    ADD COLUMN username      TEXT,
    ADD COLUMN password_hash TEXT,
    ADD COLUMN phone_number  TEXT;

-- Isi placeholder untuk baris existing (jika ada) sebelum set NOT NULL
UPDATE faskes SET username = '', password_hash = '', phone_number = '' WHERE username IS NULL;

ALTER TABLE faskes
    ALTER COLUMN username      SET NOT NULL,
    ALTER COLUMN password_hash SET NOT NULL,
    ALTER COLUMN phone_number  SET NOT NULL;

CREATE UNIQUE INDEX idx_faskes_username ON faskes(username);

COMMIT;
