-- Postgres tidak mendukung DROP VALUE pada ENUM tanpa membangun ulang tipe & menulis ulang
-- semua kolom yang memakainya. Nilai 'escalation' dibiarkan ada (DEPRECATED bila di-rollback).
-- Down ini sengaja no-op — konsisten dengan precedent 000012 (lihat docs/erd.md).
SELECT 1;
