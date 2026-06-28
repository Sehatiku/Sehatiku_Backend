-- 000008_patient_mobile_features.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_consultations_patient;
DROP TABLE IF EXISTS consultations;

-- CATATAN: ALTER TYPE ... ADD VALUE tidak bisa di-rollback di PostgreSQL.
-- Nilai 'weight' akan tetap ada di enum health_metric setelah down migration ini.

ALTER TABLE nakes DROP COLUMN IF EXISTS schedule;
ALTER TABLE nakes DROP COLUMN IF EXISTS hospital;
ALTER TABLE nakes DROP COLUMN IF EXISTS specialization;

COMMIT;
