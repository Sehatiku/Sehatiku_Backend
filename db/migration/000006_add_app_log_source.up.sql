-- 000006_add_app_log_source.up.sql
-- Pasien kini punya 2 kanal input: WhatsApp (sudah ada) dan Patient Mobile App (baru).
-- Menambah nilai 'app' ke enum log_source — lihat docs/api_guide.md & docs/redis.md
-- untuk konteks mobile app pasien.

BEGIN;

ALTER TYPE log_source ADD VALUE IF NOT EXISTS 'app';

COMMIT;
