-- full_schema_drop.sql
-- Menghapus SEMUA objek schema Sehatiku — gunakan hanya di dev/staging.
-- Urutan DROP mengikuti dependency (tabel yang di-referensikan FK dihapus terakhir).

BEGIN;

-- Aksi & komunikasi
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS escalations   CASCADE;

-- ML output & features
DROP TABLE IF EXISTS risk_scores     CASCADE;
DROP TABLE IF EXISTS daily_features  CASCADE;
DROP TABLE IF EXISTS model_versions  CASCADE;

-- Data mentah
DROP TABLE IF EXISTS lab_results CASCADE;
DROP TABLE IF EXISTS health_logs  CASCADE;

-- Tenancy & identitas
DROP TABLE IF EXISTS patient_clinical_baselines CASCADE;
DROP TABLE IF EXISTS patients CASCADE;
DROP TABLE IF EXISTS nakes    CASCADE;
DROP TABLE IF EXISTS faskes   CASCADE;

-- Enum types
DROP TYPE IF EXISTS model_type;
DROP TYPE IF EXISTS notification_status;
DROP TYPE IF EXISTS message_type;
DROP TYPE IF EXISTS recipient_role;
DROP TYPE IF EXISTS escalation_feedback;
DROP TYPE IF EXISTS escalation_status;
DROP TYPE IF EXISTS comm_channel;
DROP TYPE IF EXISTS escalation_tier;
DROP TYPE IF EXISTS scoring_mode;
DROP TYPE IF EXISTS risk_status;
DROP TYPE IF EXISTS lab_source;
DROP TYPE IF EXISTS lab_type;
DROP TYPE IF EXISTS log_source;
DROP TYPE IF EXISTS health_metric;
DROP TYPE IF EXISTS health_logger;
DROP TYPE IF EXISTS disease_type;
DROP TYPE IF EXISTS patient_sex;
DROP TYPE IF EXISTS nakes_role;
DROP TYPE IF EXISTS entity_status;
DROP TYPE IF EXISTS faskes_type;

COMMIT;
