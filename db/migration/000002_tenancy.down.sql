-- 000002_tenancy.down.sql

BEGIN;

DROP TABLE IF EXISTS patients;
DROP TABLE IF EXISTS nakes;
DROP TABLE IF EXISTS faskes;
DROP TABLE IF EXISTS patient_clinical_baselines;
COMMIT;
