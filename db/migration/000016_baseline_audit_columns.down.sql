-- 000016_baseline_audit_columns.down.sql

BEGIN;

ALTER TABLE patient_clinical_baselines
    DROP COLUMN IF EXISTS notes,
    DROP COLUMN IF EXISTS recorded_by_nakes_id;

COMMIT;
