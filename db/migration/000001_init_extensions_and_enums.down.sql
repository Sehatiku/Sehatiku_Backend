-- 000001_init_extensions_and_enums.down.sql

BEGIN;

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

-- pgcrypto sengaja TIDAK di-drop: bisa dipakai objek lain di database.

COMMIT;
