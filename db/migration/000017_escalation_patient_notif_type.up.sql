-- 000017: tambah nilai 'escalation' ke patient_notification_type agar eskalasi acute
-- bisa muncul di inbox in-app pasien (GET /api/v1/patients/notifications).
-- Catatan: ALTER TYPE ... ADD VALUE tidak dibungkus BEGIN/COMMIT (beberapa versi Postgres
-- melarang ADD VALUE di dalam blok transaksi). IF NOT EXISTS membuat migrasi idempoten.
ALTER TYPE patient_notification_type ADD VALUE IF NOT EXISTS 'escalation';
