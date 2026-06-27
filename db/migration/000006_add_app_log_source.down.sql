-- 000006_add_app_log_source.down.sql
-- Postgres tidak mendukung DROP VALUE pada enum secara native, jadi tipe direkonstruksi
-- ulang tanpa nilai 'app'. GAGAL SENGAJA (RAISE EXCEPTION) jika masih ada baris
-- health_logs dengan source = 'app' — mencegah silent data loss saat rollback.

BEGIN;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM health_logs WHERE source = 'app') THEN
        RAISE EXCEPTION 'Tidak bisa rollback: masih ada health_logs dengan source = ''app''. Hapus/migrasikan data tersebut dulu sebelum rollback.';
    END IF;
END $$;

-- DEFAULT pada kolom ini terikat ke tipe log_source — harus dilepas dulu,
-- kalau tidak DROP TYPE gagal dengan "other objects depend on it" walau
-- tidak ada baris data yang memakai nilai 'app'.
ALTER TABLE health_logs ALTER COLUMN source DROP DEFAULT;
ALTER TABLE health_logs ALTER COLUMN source TYPE TEXT;
DROP TYPE log_source;
CREATE TYPE log_source AS ENUM ('whatsapp', 'sms', 'web');
ALTER TABLE health_logs ALTER COLUMN source TYPE log_source USING source::log_source;
ALTER TABLE health_logs ALTER COLUMN source SET DEFAULT 'whatsapp'::log_source;

COMMIT;
